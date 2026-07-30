package app

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image/png"
	"net/http"
	"strings"

	"github.com/pquerna/otp"
	"roblox-community/internal/store"
)

func (s *Server) twoFactorStatus(w http.ResponseWriter, r *http.Request) {
	enabled, err := s.store.TwoFactorEnabled(currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_status_failed", "两步验证状态读取失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
}

func (s *Server) setupTwoFactor(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	settings, err := s.store.GetSiteSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_setup_failed", "两步验证设置创建失败")
		return
	}
	setup, err := s.store.BeginTwoFactorSetup(user.ID, settings.SiteName, user.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_setup_failed", "两步验证设置创建失败")
		return
	}
	key, err := otp.NewKeyFromURL(setup.URI)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_setup_failed", "两步验证二维码创建失败")
		return
	}
	image, err := key.Image(256, 256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_setup_failed", "两步验证二维码创建失败")
		return
	}
	var qrPNG bytes.Buffer
	if err := png.Encode(&qrPNG, image); err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_setup_failed", "两步验证二维码创建失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"secret":      setup.Secret,
		"otpauth_uri": setup.URI,
		"qr_data_url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrPNG.Bytes()),
	})
}

func (s *Server) confirmTwoFactor(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.store.ConfirmTwoFactor(currentUser(r).ID, strings.TrimSpace(input.Code)); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrInvalidTwoFactorCode) {
			writeError(w, status, "two_factor_code_invalid", "动态验证码不正确")
		} else {
			writeError(w, status, "two_factor_confirm_failed", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": true})
}

func (s *Server) disableTwoFactor(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := s.store.DisableTwoFactor(currentUser(r).ID, strings.TrimSpace(input.Code)); err != nil {
		writeError(w, http.StatusBadRequest, "two_factor_disable_failed", "动态验证码不正确，无法关闭两步验证")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": false})
}

func (s *Server) verifyTwoFactorLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ChallengeToken string `json:"challenge_token"`
		Code           string `json:"code"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	user, token, err := s.store.CompleteTwoFactorLogin(input.ChallengeToken, input.Code)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "two_factor_login_failed", "动态验证码不正确或登录验证已过期")
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}
