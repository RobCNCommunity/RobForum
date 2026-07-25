package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"roblox-community/internal/auth"
	"roblox-community/internal/contentmoderation"
	"roblox-community/internal/domain"
	"roblox-community/internal/store"
)

type Server struct {
	store          *store.Store
	staticDir      string
	uploadDir      string
	publicURL      string
	logger         *slog.Logger
	limiter        *requestLimiter
	moderator      contentmoderation.Service
	chatHub        *chatHub
	chatLimit      *chatConnectionLimiter
	trustedProxies trustedProxySet
}

type contextKey string

const userKey contextKey = "user"

func New(data *store.Store, staticDir, uploadDir, publicURL string, logger *slog.Logger) *Server {
	return NewWithModeration(data, staticDir, uploadDir, publicURL, logger, nil)
}

func NewWithModeration(data *store.Store, staticDir, uploadDir, publicURL string, logger *slog.Logger, moderator contentmoderation.Service) *Server {
	return &Server{store: data, staticDir: staticDir, uploadDir: uploadDir, publicURL: strings.TrimRight(publicURL, "/"), logger: logger, limiter: newRequestLimiter(), moderator: moderator, chatHub: newChatHub(), chatLimit: newChatConnectionLimiter()}
}

func (s *Server) ConfigureTrustedProxyCIDRs(value string) error {
	trusted, err := parseTrustedProxyCIDRs(value)
	if err != nil {
		return err
	}
	s.trustedProxies = trusted
	return nil
}

func (s *Server) Router() http.Handler {
	s.reconcileDeletedCommunityMedia()
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, requestTimeoutExceptStreams(2*time.Minute))
	r.Use(s.securityHeaders, s.cors, s.rateLimit, s.csrfProtection)
	r.Get("/api/v1/health", s.health)
	r.Get("/api/v1/site/settings", s.publicSiteSettings)
	r.Get("/api/v1/membership/config", s.publicMembershipConfig)
	r.Get("/api/v1/captcha/config", s.publicCaptchaConfig)
	r.Get("/api/v1/oauth/config", s.publicOAuthConfig)
	r.Get("/api/v1/oauth/start", s.oauthStart)
	r.Get("/api/v1/oauth/callback", s.oauthCallback)
	r.Get("/api/v1/users/search", s.searchUsers)
	r.Get("/api/v1/users/hot", s.hotUsers)
	r.Get("/api/v1/users/{userID}", s.getUserProfile)
	r.Get("/api/v1/search", s.searchCommunity)
	r.Get("/api/v1/media/avatars/{filename}", s.serveAvatar)
	r.Get("/api/v1/media/covers/{filename}", s.serveCover)
	r.Get("/api/v1/media/site-assets/{filename}", s.serveSiteAsset)
	r.Get("/api/v1/media/posts/{filename}", s.servePostMedia)
	r.Get("/api/v1/media/resources/{filename}", s.serveResourceMedia)
	r.Get("/api/v1/media/comments/{filename}", s.serveCommentMedia)
	r.Post("/api/v1/auth/register", s.register)
	r.Post("/api/v1/auth/register/verification", s.sendRegisterVerification)
	r.Post("/api/v1/auth/login", s.login)
	r.Post("/api/v1/auth/logout", s.authenticate(http.HandlerFunc(s.logout)).ServeHTTP)
	r.Post("/api/v1/auth/forgot-password", s.forgotPassword)
	r.Post("/api/v1/auth/reset-password", s.resetPassword)
	r.Get("/api/v1/boards", s.listBoards)
	r.Get("/api/v1/ads", s.listPublicAds)
	r.Get("/api/v1/notices", s.listPublicNotices)
	r.Get("/api/v1/posts", s.listPosts)
	r.Get("/api/v1/posts/{postID}", s.getPost)
	r.Get("/api/v1/posts/{postID}/comments", s.listComments)
	r.Get("/api/v1/resources", s.listPublicResources)
	r.Get("/api/v1/resources/{resourceID}", s.getPublicResource)
	r.Get("/api/v1/resources/{resourceID}/download", s.downloadResource)
	r.Post("/api/v1/payment/callback", s.paymentCallback)
	r.Group(func(r chi.Router) {
		r.Use(s.authenticate)
		r.Get("/api/v1/me", s.me)
		r.Patch("/api/v1/me/profile", s.updateMyProfile)
		r.Post("/api/v1/me/avatar", s.uploadMyAvatar)
		r.Post("/api/v1/me/cover", s.uploadMyCover)
		r.Delete("/api/v1/me/cover", s.deleteMyCover)
		r.Get("/api/v1/me/verification-applications", s.listMyVerificationApplications)
		r.Post("/api/v1/me/verification-applications", s.createVerificationApplication)
		r.Post("/api/v1/posts", s.createPost)
		r.Post("/api/v1/posts/{postID}/comments", s.createComment)
		r.Post("/api/v1/posts/{postID}/report", s.createPostReport)
		r.Delete("/api/v1/comments/{commentID}", s.deleteComment)
		r.Post("/api/v1/comments/{commentID}/report", s.createCommentReport)
		r.Get("/api/v1/posts/{postID}/like", s.postLikeStatus)
		r.Post("/api/v1/posts/{postID}/like", s.togglePostLike)
		r.Get("/api/v1/comments/{commentID}/like", s.commentLikeStatus)
		r.Post("/api/v1/comments/{commentID}/like", s.toggleCommentLike)
		r.Get("/api/v1/users/{userID}/block", s.userBlockStatus)
		r.Post("/api/v1/users/{userID}/block", s.toggleUserBlock)
		r.Delete("/api/v1/users/{userID}/block", s.toggleUserBlock)
		r.Post("/api/v1/users/{userID}/report", s.createProfileReport)
		r.Get("/api/v1/users/{userID}/follow", s.followStatus)
		r.Post("/api/v1/users/{userID}/follow", s.followUser)
		r.Delete("/api/v1/users/{userID}/follow", s.unfollowUser)
		r.Get("/api/v1/posts/{postID}/bookmark", s.bookmarkStatus)
		r.Post("/api/v1/posts/{postID}/bookmark", s.toggleBookmark)
		r.Get("/api/v1/posts/{postID}/repost", s.repostStatus)
		r.Post("/api/v1/posts/{postID}/repost", s.toggleRepost)
		r.Get("/api/v1/me/feed/following", s.followingFeed)
		r.Get("/api/v1/me/bookmarks", s.myBookmarks)
		r.Get("/api/v1/me/notifications", s.notifications)
		r.Get("/api/v1/me/notifications/unread-count", s.unreadNotifications)
		r.Post("/api/v1/me/notifications/read", s.markNotificationsRead)
		r.Get("/api/v1/me/conversations", s.listConversations)
		r.Get("/api/v1/me/conversation-invites", s.listConversationInvites)
		r.Post("/api/v1/conversations", s.createConversation)
		r.Post("/api/v1/conversation-invites/{conversationID}/respond", s.respondConversationInvite)
		r.Post("/api/v1/conversation-invite-links/join", s.joinConversationByInvite)
		r.Get("/api/v1/conversations/{conversationID}/members", s.listConversationMembers)
		r.Post("/api/v1/conversations/{conversationID}/members", s.inviteConversationMembers)
		r.Delete("/api/v1/conversations/{conversationID}/members/{userID}", s.removeConversationMember)
		r.Patch("/api/v1/conversations/{conversationID}", s.updateConversation)
		r.Delete("/api/v1/conversations/{conversationID}", s.deleteConversation)
		r.Post("/api/v1/conversations/{conversationID}/leave", s.leaveConversation)
		r.Post("/api/v1/conversations/{conversationID}/invite-link", s.createConversationInviteLink)
		r.Delete("/api/v1/conversations/{conversationID}/invite-link", s.revokeConversationInviteLink)
		r.Post("/api/v1/conversations/{conversationID}/remind", s.remindGroupInvitees)
		r.Get("/api/v1/conversations/{conversationID}/messages", s.listMessages)
		r.Post("/api/v1/conversations/{conversationID}/messages", s.createMessage)
		r.Get("/api/v1/conversations/{conversationID}/stream", s.streamConversationMessages)
		r.Post("/api/v1/resources", s.createResource)
		r.Get("/api/v1/me/resources", s.listMyResources)
		r.Post("/api/v1/resources/{resourceID}/orders", s.createResourceOrder)
		r.Get("/api/v1/me/orders", s.listMyOrders)
		r.Get("/api/v1/me/wallet", s.myWallet)
		r.Get("/api/v1/me/membership", s.myMembership)
		r.Post("/api/v1/me/membership/subscribe", s.subscribeMembership)
		r.Post("/api/v1/me/wallet/top-ups", s.createWalletTopUp)
		r.Post("/api/v1/me/wallet/redeem", s.redeemWalletCode)
		r.Post("/api/v1/resources/{resourceID}/wallet-order", s.createWalletResourceOrder)
		r.Get("/api/v1/me/creator/balance", s.creatorBalance)
		r.Post("/api/v1/me/creator/payouts", s.requestCreatorPayout)
		r.Get("/api/v1/me/creator/payouts", s.listCreatorPayouts)
		r.Post("/api/v1/roblox/binding/challenge", s.createRobloxChallenge)
		r.Post("/api/v1/roblox/binding/verify", s.verifyRobloxChallenge)
	})
	r.Group(func(r chi.Router) {
		r.Use(s.authenticate, s.adminOnly)
		r.Get("/api/v1/admin/site", s.adminSiteSettings)
		r.Put("/api/v1/admin/site", s.updateSiteSettings)
		r.Post("/api/v1/admin/site/verification-badge", s.uploadVerificationBadge)
		r.Delete("/api/v1/admin/site/verification-badge", s.deleteVerificationBadge)
		r.Get("/api/v1/admin/membership", s.adminMembership)
		r.Put("/api/v1/admin/membership", s.updateAdminMembership)
		r.Post("/api/v1/admin/membership/tiers", s.createMembershipTier)
		r.Put("/api/v1/admin/membership/tiers/{tierID}", s.updateMembershipTier)
		r.Delete("/api/v1/admin/membership/tiers/{tierID}", s.deleteMembershipTier)
		r.Post("/api/v1/admin/membership/tiers/{tierID}/badge", s.uploadMembershipTierBadge)
		r.Delete("/api/v1/admin/membership/tiers/{tierID}/badge", s.deleteMembershipTierBadge)
		r.Post("/api/v1/admin/membership/badge", s.uploadMembershipBadge)
		r.Delete("/api/v1/admin/membership/badge", s.deleteMembershipBadge)
		r.Get("/api/v1/admin/smtp", s.adminSMTP)
		r.Put("/api/v1/admin/smtp", s.updateSMTP)
		r.Post("/api/v1/admin/smtp/test", s.testSMTP)
		r.Get("/api/v1/admin/captcha", s.adminCaptcha)
		r.Put("/api/v1/admin/captcha", s.updateCaptcha)
		r.Get("/api/v1/admin/oauth", s.adminOAuthConfig)
		r.Put("/api/v1/admin/oauth", s.updateOAuthConfig)
		r.Get("/api/v1/admin/resources", s.listAdminResources)
		r.Patch("/api/v1/admin/resources/{resourceID}", s.reviewResource)
		r.Get("/api/v1/admin/resources/{resourceID}/download", s.downloadResourceForAdmin)
		r.Get("/api/v1/admin/payment", s.adminPayment)
		r.Put("/api/v1/admin/payment", s.updatePayment)
		r.Get("/api/v1/admin/wallet/redeem-codes", s.listAdminRedeemCodes)
		r.Post("/api/v1/admin/wallet/redeem-codes", s.createAdminRedeemCode)
		r.Post("/api/v1/admin/wallet/redeem-codes/{codeID}/revoke", s.revokeAdminRedeemCode)
		r.Get("/api/v1/admin/payouts", s.listAdminPayouts)
		r.Patch("/api/v1/admin/payouts/{payoutID}", s.reviewAdminPayout)
		r.Get("/api/v1/admin/verifications", s.listAdminVerificationApplications)
		r.Patch("/api/v1/admin/verifications/{applicationID}", s.reviewVerificationApplication)
		r.Get("/api/v1/admin/users", s.listAdminUsers)
		r.Patch("/api/v1/admin/users/{userID}/status", s.updateAdminUserStatus)
		r.Delete("/api/v1/admin/users/{userID}", s.deleteAdminUser)
		r.Get("/api/v1/admin/posts", s.listAdminPosts)
		r.Patch("/api/v1/admin/posts/{postID}/moderate", s.moderateAdminPost)
		r.Get("/api/v1/admin/reports", s.listAdminContentReports)
		r.Patch("/api/v1/admin/reports/{reportID}", s.reviewAdminContentReport)
		r.Get("/api/v1/admin/ads", s.listAdminAds)
		r.Post("/api/v1/admin/ads", s.createAdminAd)
		r.Put("/api/v1/admin/ads/{adID}", s.updateAdminAd)
		r.Delete("/api/v1/admin/ads/{adID}", s.deleteAdminAd)
		r.Get("/api/v1/admin/notices", s.listAdminNotices)
		r.Post("/api/v1/admin/notices", s.createAdminNotice)
		r.Put("/api/v1/admin/notices/{noticeID}", s.updateAdminNotice)
		r.Delete("/api/v1/admin/notices/{noticeID}", s.deleteAdminNotice)
		r.Patch("/api/v1/admin/posts/{postID}/pin", s.pinPost)
	})
	s.mountFrontend(r)
	return r
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; img-src 'self' data: https:; font-src 'self' data:; style-src 'self' 'unsafe-inline' https://static.geetest.com https://*.geetest.com; script-src 'self' https://static.cloudflareinsights.com https://static.geetest.com https://*.geetest.com https://*.geevisit.com https://*.gsensebot.com; connect-src 'self' https:; frame-src 'self' https://*.geetest.com https://*.geevisit.com https://*.gsensebot.com")
		if requestScheme(r, s.trustedProxies) == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://127.0.0.1:5173" || origin == "http://localhost:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-CSRF-Token")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) mountFrontend(r *chi.Mux) {
	index := filepath.Join(s.staticDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		return
	}
	files := http.FileServer(http.Dir(s.staticDir))
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "not_found", "接口不存在")
			return
		}
		cleanPath := strings.TrimPrefix(pathpkg.Clean("/"+req.URL.Path), "/")
		localPath := filepath.Join(s.staticDir, filepath.FromSlash(cleanPath))
		relative, relErr := filepath.Rel(s.staticDir, localPath)
		if relErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
				if strings.HasPrefix(req.URL.Path, "/assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "public, max-age=3600")
				}
				files.ServeHTTP(w, req)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		http.ServeFile(w, req, index)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "数据库暂不可用")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) publicSiteSettings(w http.ResponseWriter, _ *http.Request) {
	settings, err := s.store.GetSiteSettings()
	if err != nil {
		writeError(w, 500, "settings_unavailable", "站点配置暂不可用")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) publicCaptchaConfig(w http.ResponseWriter, _ *http.Request) {
	config, err := s.store.GetCaptchaConfig()
	if err != nil {
		writeError(w, 500, "captcha_unavailable", "验证码配置暂不可用")
		return
	}
	config.MaskedSecret = ""
	writeJSON(w, http.StatusOK, map[string]any{"enabled": config.Enabled, "provider": config.Provider, "site_key": config.SiteKey, "adapter_status": config.AdapterStatus})
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email        string `json:"email"`
		Password     string `json:"password"`
		DisplayName  string `json:"display_name"`
		CaptchaToken string `json:"captcha_token"`
		EmailCode    string `json:"email_code"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	settings, err := s.store.GetSiteSettings()
	if err != nil {
		writeError(w, 500, "settings_unavailable", "站点配置暂不可用")
		return
	}
	if !settings.AllowRegister {
		writeError(w, 403, "register_disabled", "当前暂未开放注册")
		return
	}
	if err := validateEmail(input.Email, settings.AllowedEmailDomains); err != nil {
		writeError(w, 400, "invalid_email", err.Error())
		return
	}
	// When email verification is enabled, the one-time code was issued only
	// after a successful CAPTCHA challenge. Requiring another challenge here
	// would consume a second GT4 token without adding a new trust boundary.
	if registrationCaptchaRequired(settings) {
		if err := s.verifyCaptcha(r, input.CaptchaToken); err != nil {
			writeError(w, 400, "captcha_required", err.Error())
			return
		}
	}
	user, token, err := s.store.RegisterUser(input.Email, input.Password, input.DisplayName, input.EmailCode, settings.RequireEmailVerification)
	if err != nil {
		writeError(w, 400, "register_failed", err.Error())
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, 201, map[string]any{"user": user})
}

func registrationCaptchaRequired(settings domain.SiteSettings) bool {
	return !settings.RequireEmailVerification
}

func (s *Server) sendRegisterVerification(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email        string `json:"email"`
		CaptchaToken string `json:"captcha_token"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	settings, err := s.store.GetSiteSettings()
	if err != nil {
		writeError(w, 500, "settings_unavailable", "站点配置暂不可用")
		return
	}
	if !settings.RequireEmailVerification {
		writeError(w, 409, "verification_disabled", "当前未启用邮箱验证码注册")
		return
	}
	if err := validateEmail(input.Email, settings.AllowedEmailDomains); err != nil {
		writeError(w, 400, "invalid_email", err.Error())
		return
	}
	if err := s.verifyCaptcha(r, input.CaptchaToken); err != nil {
		writeError(w, 400, "captcha_required", err.Error())
		return
	}
	if _, err := s.store.GetUserByEmail(input.Email); err == nil {
		writeError(w, 409, "email_exists", "该邮箱已经注册")
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		writeError(w, 500, "user_lookup_failed", "邮箱状态暂时无法确认")
		return
	}
	code, validity, err := s.store.CreateEmailVerification(input.Email)
	if err != nil {
		writeError(w, 429, "verification_cooldown", err.Error())
		return
	}
	body := fmt.Sprintf("你好，你的注册验证码是：%s\r\n\r\n验证码将在 %d 分钟后失效，请勿转发给他人。\r\n", code, int(validity.Minutes()))
	if err := s.sendEmail(r, input.Email, "注册验证码", body); err != nil {
		s.logger.Error("send registration verification email failed", "recipient", input.Email, "error", err)
		if discardErr := s.store.DiscardEmailVerification(input.Email); discardErr != nil {
			s.logger.Error("discard email verification after delivery failure failed", "error", discardErr)
		}
		writeError(w, 502, "smtp_send_failed", "验证码邮件发送失败，请检查 SMTP 配置")
		return
	}
	writeJSON(w, 200, map[string]any{"sent": true, "expires_in": int(validity.Seconds())})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email        string `json:"email"`
		Password     string `json:"password"`
		CaptchaToken string `json:"captcha_token"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.verifyCaptcha(r, input.CaptchaToken); err != nil {
		writeError(w, 400, "captcha_required", err.Error())
		return
	}
	user, err := s.store.VerifyPassword(input.Email, input.Password)
	if err != nil {
		if errors.Is(err, store.ErrInvalidCredentials) {
			writeError(w, 401, "invalid_credentials", "邮箱或密码错误")
		} else {
			writeError(w, 500, "login_failed", "登录服务暂时不可用")
		}
		return
	}
	token, err := s.store.IssueSession(user.ID)
	if err != nil {
		writeError(w, 500, "session_failed", "登录会话创建失败")
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, 200, map[string]any{"user": user})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	var deleteErr error
	for _, token := range sessionTokens(r) {
		if err := s.store.DeleteSession(token); err != nil {
			s.logger.Error("delete session on logout failed", "error", err)
			deleteErr = err
		}
	}
	if deleteErr != nil {
		s.clearSessionCookies(w, r)
		writeError(w, 500, "logout_failed", "退出登录失败，请稍后重试")
		return
	}
	s.clearSessionCookies(w, r)
	writeJSON(w, 200, map[string]any{"logged_out": true})
}

func (s *Server) clearSessionCookies(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "roblox_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secureCookies(r), SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: "", Path: "/", MaxAge: -1, Secure: s.secureCookies(r), SameSite: http.SameSiteLaxMode})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, currentUser(r)) }

func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email        string `json:"email"`
		CaptchaToken string `json:"captcha_token"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.verifyCaptcha(r, input.CaptchaToken); err != nil {
		writeError(w, 400, "captcha_required", err.Error())
		return
	}
	message := "如果该邮箱已注册，重置密码的邮件会发送到你的邮箱。"
	token, user, err := s.store.CreateResetToken(input.Email)
	if err == nil {
		if err := s.sendResetEmail(r, user.Email, token); err != nil {
			s.logger.Error("send reset email failed", "error", err)
			if revokeErr := s.store.RevokeResetToken(token); revokeErr != nil {
				s.logger.Error("revoke reset token after email failure failed", "error", revokeErr)
			}
		}
	}
	writeJSON(w, 200, map[string]any{"message": message})
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := auth.ValidatePassword(input.Password); err != nil {
		writeError(w, 400, "weak_password", err.Error())
		return
	}
	if err := s.store.ResetPassword(strings.TrimSpace(input.Token), input.Password); err != nil {
		writeError(w, 400, "reset_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"reset": true})
}

func (s *Server) listBoards(w http.ResponseWriter, _ *http.Request) {
	result, err := s.store.ListBoards()
	if err != nil {
		writeError(w, 500, "boards_failed", "板块加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func recommendedPostsRequested(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("feed")), "for-you")
}

func requestedPostBoard(r *http.Request) string {
	if recommendedPostsRequested(r) {
		return ""
	}
	board := strings.TrimSpace(r.URL.Query().Get("board"))
	if strings.EqualFold(board, "all") {
		return ""
	}
	return board
}

func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	board := requestedPostBoard(r)
	if r.URL.Query().Get("paged") == "1" {
		limit, offset := paginationParams(r, 30, 50)
		var (
			result domain.PostPage
			err    error
		)
		if recommendedPostsRequested(r) {
			result, err = s.store.ListRecommendedPostsPage(limit, offset)
		} else {
			result, err = s.store.ListPostsPage(board, r.URL.Query().Get("q"), limit, offset)
		}
		if err != nil {
			writeError(w, 500, "posts_failed", "帖子加载失败")
			return
		}
		if viewer, _, sessionErr := s.userBySessionCookies(r); sessionErr == nil {
			if err := s.store.MarkPostsLikedBy(viewer.ID, result.Items); err != nil {
				s.logger.Error("load post like state failed", "user_id", viewer.ID, "error", err)
				writeError(w, 500, "posts_failed", "帖子加载失败")
				return
			}
		}
		writeJSON(w, 200, result)
		return
	}
	var (
		result []domain.Post
		err    error
	)
	if recommendedPostsRequested(r) {
		result, err = s.store.ListRecommendedPosts(30)
	} else {
		result, err = s.store.ListPosts(board, r.URL.Query().Get("q"), 30)
	}
	if err != nil {
		writeError(w, 500, "posts_failed", "帖子加载失败")
		return
	}
	// This endpoint is public, but a valid session lets the timeline expose
	// viewer-specific interaction state without making anonymous access fail.
	if viewer, _, sessionErr := s.userBySessionCookies(r); sessionErr == nil {
		if err := s.store.MarkPostsLikedBy(viewer.ID, result); err != nil {
			s.logger.Error("load post like state failed", "user_id", viewer.ID, "error", err)
			writeError(w, 500, "posts_failed", "帖子加载失败")
			return
		}
	}
	writeJSON(w, 200, result)
}

func (s *Server) getPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "postID"), 10, 64)
	item, err := s.store.GetPost(id)
	if err != nil {
		writeError(w, 404, "post_not_found", "帖子不存在")
		return
	}
	writeJSON(w, 200, item)
}

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		s.createPostWithMedia(w, r)
		return
	}
	var input struct {
		BoardID int64    `json:"board_id"`
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	machineApproved := s.postMachineApproved(r, postModerationText(input.Title, input.Content, input.Tags))
	item, err := s.store.CreateMachineModeratedPostWithTagsAndMedia(currentUser(r).ID, input.BoardID, input.Title, input.Content, input.Tags, nil, machineApproved)
	if err != nil {
		writeError(w, 400, "post_failed", err.Error())
		return
	}
	writeJSON(w, 201, item)
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "postID"), 10, 64)
	result, err := s.store.ListComments(id)
	if err != nil {
		writeError(w, 500, "comments_failed", "评论加载失败")
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) createComment(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "postID"), 10, 64)
	user := currentUser(r)
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxCommentMediaCount*maxCommentMediaUpload+(1<<20))
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "comment_upload_invalid", "评论媒体过大或表单格式无效")
			return
		}
		defer r.MultipartForm.RemoveAll()
		content := strings.TrimSpace(r.FormValue("content"))
		var parentID *int64
		if rawParentID := strings.TrimSpace(r.FormValue("parent_id")); rawParentID != "" {
			value, parseErr := strconv.ParseInt(rawParentID, 10, 64)
			if parseErr != nil || value < 1 {
				writeError(w, http.StatusBadRequest, "comment_parent_invalid", "回复目标无效")
				return
			}
			parentID = &value
		}
		files := r.MultipartForm.File["files"]
		if len(files) == 0 {
			files = r.MultipartForm.File["file"]
		}
		if len(files) > maxCommentMediaCount {
			writeError(w, http.StatusBadRequest, "comment_media_too_many", "每条评论最多上传 4 个媒体文件")
			return
		}
		if content == "" && len(files) == 0 {
			writeError(w, http.StatusBadRequest, "comment_invalid", "评论内容或媒体不能为空")
			return
		}
		if content != "" && !s.approveContent(w, r, "comment", content) {
			return
		}
		media := make([]store.CommentMediaInput, 0, len(files))
		savedPaths := make([]string, 0, len(files))
		cleanup := func() {
			for _, path := range savedPaths {
				_ = os.Remove(path)
			}
		}
		for _, header := range files {
			item, path, saveErr := s.saveCommentMedia(header)
			if saveErr != nil {
				cleanup()
				writeError(w, http.StatusBadRequest, "comment_media_invalid", saveErr.Error())
				return
			}
			savedPaths = append(savedPaths, path)
			if isImageMedia(item.MIMEType) && !s.approveImagePath(w, r, "comment", path) {
				cleanup()
				return
			}
			media = append(media, item)
		}
		item, err := s.store.CreateCommentWithMediaAndParent(user.ID, id, parentID, content, media)
		if err != nil {
			cleanup()
			switch {
			case errors.Is(err, store.ErrCommentInvalid):
				writeError(w, http.StatusBadRequest, "comment_invalid", "评论内容或图片无效")
			case errors.Is(err, store.ErrCommentAuthorUnavailable):
				writeError(w, http.StatusForbidden, "comment_author_unavailable", "当前账号无法发表评论")
			case errors.Is(err, store.ErrPostUnavailable):
				writeError(w, http.StatusNotFound, "post_unavailable", "帖子不存在或已停止评论")
			case errors.Is(err, store.ErrParentCommentUnavailable):
				writeError(w, http.StatusNotFound, "comment_parent_unavailable", "要回复的评论不存在")
			default:
				s.logger.Error("create comment with media failed", "request_id", middleware.GetReqID(r.Context()), "user_id", user.ID, "post_id", id, "error", err)
				writeError(w, http.StatusInternalServerError, "comment_failed", "评论发布失败，请稍后重试")
			}
			return
		}
		writeJSON(w, http.StatusCreated, item)
		return
	}
	var input struct {
		Content  string `json:"content"`
		ParentID *int64 `json:"parent_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !s.approveContent(w, r, "comment", input.Content) {
		return
	}
	item, err := s.store.CreateCommentWithMediaAndParent(user.ID, id, input.ParentID, input.Content, nil)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrCommentInvalid):
			writeError(w, http.StatusBadRequest, "comment_invalid", "评论内容不能为空、超过 5000 字或包含无效字符")
		case errors.Is(err, store.ErrCommentAuthorUnavailable):
			writeError(w, http.StatusForbidden, "comment_author_unavailable", "当前账号无法发表评论")
		case errors.Is(err, store.ErrPostUnavailable):
			writeError(w, http.StatusNotFound, "post_unavailable", "帖子不存在或已停止评论")
		case errors.Is(err, store.ErrParentCommentUnavailable):
			writeError(w, http.StatusNotFound, "comment_parent_unavailable", "要回复的评论不存在")
		default:
			s.logger.Error("create comment failed", "request_id", middleware.GetReqID(r.Context()), "user_id", user.ID, "post_id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "comment_failed", "评论发布失败，请稍后重试")
		}
		return
	}
	writeJSON(w, 201, item)
}

func (s *Server) createRobloxChallenge(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented, "roblox_adapter_pending", "Roblox 身份验证尚未接入，当前不会生成无效挑战")
}
func (s *Server) verifyRobloxChallenge(w http.ResponseWriter, _ *http.Request) {
	writeError(w, 501, "roblox_adapter_pending", "Roblox 公共资料验证适配器正在预留，暂未完成真实校验")
}

func (s *Server) adminSiteSettings(w http.ResponseWriter, _ *http.Request) {
	result, err := s.store.GetSiteSettings()
	if err != nil {
		writeError(w, 500, "settings_failed", "配置读取失败")
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) updateSiteSettings(w http.ResponseWriter, r *http.Request) {
	var input domain.SiteSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.RequireEmailVerification {
		if _, err := s.store.SMTPDeliveryConfig(); err != nil || !smtpReady(s.store) {
			writeError(w, 400, "smtp_required", "启用邮箱验证码前，请先配置并启用 SMTP")
			return
		}
	}
	result, err := s.store.UpdateSiteSettings(currentUser(r).ID, input)
	if err != nil {
		writeError(w, 400, "settings_invalid", err.Error())
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) adminSMTP(w http.ResponseWriter, _ *http.Request) {
	result, err := s.store.GetSMTPConfig()
	if err != nil {
		writeError(w, 500, "smtp_failed", "SMTP 配置读取失败")
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) updateSMTP(w http.ResponseWriter, r *http.Request) {
	var input domain.SMTPConfig
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.store.UpdateSMTPConfig(currentUser(r).ID, input)
	if err != nil {
		writeError(w, 400, "smtp_invalid", err.Error())
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) testSMTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := validateEmail(input.Email, nil); err != nil {
		writeError(w, 400, "invalid_email", "测试收件邮箱无效")
		return
	}
	if err := s.sendEmail(r, input.Email, "SMTP 测试邮件", "SMTP 配置测试成功。\r\n"); err != nil {
		s.logger.Error("send SMTP test email failed", "recipient", input.Email, "error", err)
		writeError(w, 502, "smtp_send_failed", "测试邮件发送失败，请检查 SMTP 配置")
		return
	}
	writeJSON(w, 200, map[string]any{"sent": true})
}
func (s *Server) adminCaptcha(w http.ResponseWriter, _ *http.Request) {
	result, err := s.store.GetCaptchaConfig()
	if err != nil {
		writeError(w, 500, "captcha_failed", "验证码配置读取失败")
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) updateCaptcha(w http.ResponseWriter, r *http.Request) {
	var input domain.CaptchaConfig
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.store.UpdateCaptchaConfig(currentUser(r).ID, input)
	if err != nil {
		writeError(w, 400, "captcha_invalid", err.Error())
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(sessionTokens(r)) == 0 {
			writeError(w, 401, "unauthorized", "请先登录")
			return
		}
		user, _, err := s.userBySessionCookies(r)
		if err != nil {
			writeError(w, 401, "unauthorized", "登录已过期，请重新登录")
			return
		}
		if _, err := r.Cookie(csrfCookieName); err != nil {
			s.setCSRFCookie(w, r)
		}
		next.ServeHTTP(w, r.WithContext(withUser(r.Context(), user)))
	})
}

func (s *Server) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r).Role != "admin" {
			writeError(w, 403, "forbidden", "需要管理员权限")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) verifyCaptcha(r *http.Request, token string) error {
	config, err := s.store.CaptchaDeliveryConfig()
	if err != nil {
		return err
	}
	if !config.Enabled {
		return nil
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("请完成验证码")
	}
	if config.AdapterStatus != "ready" || config.Provider != "gt4" {
		return errors.New("验证码配置不完整")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	if err := validateGT4(r.Context(), client, config, token); err != nil {
		s.logger.Warn("GT4 captcha validation failed", "error", err, "client_ip", requestClientIP(r, s.trustedProxies))
		return errors.New("人机验证失败，请重新验证")
	}
	return nil
}

type gt4Token struct {
	CaptchaID     string `json:"captcha_id"`
	LotNumber     string `json:"lot_number"`
	CaptchaOutput string `json:"captcha_output"`
	PassToken     string `json:"pass_token"`
	GenTime       string `json:"gen_time"`
}

func validateGT4(ctx context.Context, client *http.Client, config domain.CaptchaConfig, token string) error {
	if len(token) > 16384 {
		return errors.New("captcha token is too long")
	}
	var payload gt4Token
	decoder := json.NewDecoder(strings.NewReader(token))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("decode captcha token: %w", err)
	}
	payload.CaptchaID = strings.TrimSpace(payload.CaptchaID)
	if payload.CaptchaID != "" && payload.CaptchaID != config.SiteKey {
		return errors.New("captcha token ID does not match configured captcha ID")
	}
	values := []*string{&payload.LotNumber, &payload.CaptchaOutput, &payload.PassToken, &payload.GenTime}
	for _, value := range values {
		*value = strings.TrimSpace(*value)
		if *value == "" || len(*value) > 4096 {
			return errors.New("captcha token fields are invalid")
		}
	}
	mac := hmac.New(sha256.New, []byte(config.Secret))
	_, _ = mac.Write([]byte(payload.LotNumber))
	form := url.Values{
		"captcha_id":     {config.SiteKey},
		"lot_number":     {payload.LotNumber},
		"captcha_output": {payload.CaptchaOutput},
		"pass_token":     {payload.PassToken},
		"gen_time":       {payload.GenTime},
		"sign_token":     {hex.EncodeToString(mac.Sum(nil))},
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil {
		return err
	}
	query := endpoint.Query()
	query.Set("captcha_id", config.SiteKey)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("captcha provider returned HTTP %d", response.StatusCode)
	}
	var result struct {
		Result string `json:"result"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&result); err != nil {
		return fmt.Errorf("decode captcha provider response: %w", err)
	}
	if result.Result != "success" {
		return fmt.Errorf("captcha rejected: %s", result.Reason)
	}
	return nil
}

func (s *Server) sendResetEmail(r *http.Request, recipient, token string) error {
	settings, err := s.store.GetSiteSettings()
	if err != nil {
		return err
	}
	base := strings.TrimRight(settings.PublicURL, "/")
	if base == "" {
		base = s.publicURL
	}
	if base == "" {
		return errors.New("public URL must be configured before sending reset links")
	}
	link := base + "/reset-password?token=" + token
	return s.sendEmail(r, recipient, "重置你的社区密码", fmt.Sprintf("你好，\r\n\r\n请打开下面的链接重置密码：\r\n%s\r\n\r\n链接 30 分钟内有效。\r\n", link))
}

func (s *Server) sendEmail(_ *http.Request, recipient, subject, body string) error {
	config, err := s.store.SMTPDeliveryConfig()
	if err != nil {
		return err
	}
	if !config.Enabled || config.Host == "" || config.FromEmail == "" {
		return errors.New("SMTP is disabled or incomplete")
	}
	if err := validateEmail(recipient, nil); err != nil {
		return err
	}
	from := (&mail.Address{Name: config.FromName, Address: config.FromEmail}).String()
	message := strings.Join([]string{"From: " + from, "To: " + recipient, "Subject: " + mime.QEncoding.Encode("UTF-8", subject), "MIME-Version: 1.0", "Content-Type: text/plain; charset=UTF-8", "Content-Transfer-Encoding: 8bit", "", body}, "\r\n")
	address := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	conn, err := net.DialTimeout("tcp", address, 15*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(45 * time.Second)); err != nil {
		return err
	}
	var client *smtp.Client
	if config.TLSMode == "tls" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: config.Host, MinVersion: tls.VersionTLS12})
		if err := tlsConn.Handshake(); err != nil {
			return err
		}
		client, err = smtp.NewClient(tlsConn, config.Host)
	} else {
		client, err = smtp.NewClient(conn, config.Host)
	}
	if err != nil {
		return err
	}
	defer client.Quit()
	if config.TLSMode == "starttls" {
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			return errors.New("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: config.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if config.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", config.Username, config.Password, config.Host)); err != nil {
			return err
		}
	}
	if err := client.Mail(config.FromEmail); err != nil {
		return err
	}
	if err := client.Rcpt(recipient); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte(message)); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}

func smtpReady(data *store.Store) bool {
	config, err := data.SMTPDeliveryConfig()
	return err == nil && config.Enabled && config.Host != "" && config.FromEmail != ""
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{Name: "roblox_session", Value: token, Path: "/", MaxAge: 30 * 24 * 3600, HttpOnly: true, Secure: s.secureCookies(r), SameSite: http.SameSiteLaxMode})
	s.setCSRFCookie(w, r)
}

func sessionTokens(r *http.Request) []string {
	seen := make(map[string]struct{})
	tokens := make([]string, 0, 1)
	for _, cookie := range r.Cookies() {
		if cookie.Name != "roblox_session" || strings.TrimSpace(cookie.Value) == "" {
			continue
		}
		if _, ok := seen[cookie.Value]; ok {
			continue
		}
		seen[cookie.Value] = struct{}{}
		tokens = append(tokens, cookie.Value)
	}
	return tokens
}

func resolveSessionUser(r *http.Request, lookup func(string) (domain.User, error)) (domain.User, string, error) {
	var lastErr error
	for _, token := range sessionTokens(r) {
		user, err := lookup(token)
		if err == nil {
			return user, token, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("session missing")
	}
	return domain.User{}, "", lastErr
}

func (s *Server) userBySessionCookies(r *http.Request) (domain.User, string, error) {
	return resolveSessionUser(r, s.store.UserBySession)
}

func sessionToken(r *http.Request) string {
	if tokens := sessionTokens(r); len(tokens) > 0 {
		return tokens[0]
	}
	return ""
}
func validateEmail(raw string, allowlist []string) error {
	email := strings.ToLower(strings.TrimSpace(raw))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return errors.New("请输入有效的邮箱地址")
	}
	if len(email) > 254 {
		return errors.New("邮箱地址过长")
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 || len(parts[0]) < 1 || len([]byte(parts[0])) > 64 {
		return errors.New("邮箱前缀长度无效")
	}
	if len(allowlist) == 0 {
		return nil
	}
	for _, allowed := range allowlist {
		if strings.EqualFold(parts[1], allowed) {
			return nil
		}
	}
	return errors.New("该邮箱服务商不在允许列表中")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if contentType := strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]); contentType != "" && contentType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "content_type_invalid", "请求必须使用 application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, 400, "invalid_json", "请求 JSON 无效")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, 400, "invalid_json", "请求 JSON 只能包含一个对象")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
