package app

import (
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/domain"
)

const maxMembershipBadgeUpload = 2 << 20

func (s *Server) publicMembershipConfig(w http.ResponseWriter, _ *http.Request) {
	config, err := s.store.GetMembershipSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "membership_config_failed", "会员配置暂不可用")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) myMembership(w http.ResponseWriter, r *http.Request) {
	summary, err := s.store.GetMembershipSummary(currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "membership_failed", "会员信息加载失败")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) subscribeMembership(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TierID int64  `json:"tier_id"`
		Plan   string `json:"plan"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	// Compatibility for a cached pre-tier frontend: its only possible target
	// is the first configured tier.
	if input.TierID == 0 {
		config, err := s.store.GetMembershipSettings()
		if err != nil || len(config.Tiers) == 0 {
			writeError(w, http.StatusBadRequest, "membership_purchase_failed", "会员阶层不可用")
			return
		}
		input.TierID = config.Tiers[0].ID
	}
	summary, err := s.store.SubscribeMembership(currentUser(r).ID, input.TierID, input.Plan)
	if err != nil {
		writeError(w, http.StatusBadRequest, "membership_purchase_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, summary)
}

func (s *Server) adminMembership(w http.ResponseWriter, _ *http.Request) {
	config, err := s.store.GetMembershipSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "membership_config_failed", "会员配置读取失败")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) updateAdminMembership(w http.ResponseWriter, r *http.Request) {
	var input domain.MembershipSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	config, err := s.store.UpdateMembershipSettings(currentUser(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "membership_config_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) createMembershipTier(w http.ResponseWriter, r *http.Request) {
	var input domain.MembershipTier
	if !decodeJSON(w, r, &input) {
		return
	}
	tier, err := s.store.CreateMembershipTier(currentUser(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "membership_tier_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, tier)
}

func (s *Server) updateMembershipTier(w http.ResponseWriter, r *http.Request) {
	tierID, _ := strconv.ParseInt(chi.URLParam(r, "tierID"), 10, 64)
	var input domain.MembershipTier
	if !decodeJSON(w, r, &input) {
		return
	}
	tier, err := s.store.UpdateMembershipTier(currentUser(r).ID, tierID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "membership_tier_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tier)
}

func (s *Server) deleteMembershipTier(w http.ResponseWriter, r *http.Request) {
	tierID, _ := strconv.ParseInt(chi.URLParam(r, "tierID"), 10, 64)
	tier, err := s.store.DeleteMembershipTier(currentUser(r).ID, tierID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "membership_tier_delete_failed", err.Error())
		return
	}
	removeLocalMembershipBadge(s, tier.BadgeURL)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": tier.ID})
}

// uploadMembershipBadge is retained for cached clients and updates the first
// tier. New clients use the tier-specific endpoint below.
func (s *Server) uploadMembershipBadge(w http.ResponseWriter, r *http.Request) {
	config, err := s.store.GetMembershipSettings()
	if err != nil || len(config.Tiers) == 0 {
		writeError(w, http.StatusBadRequest, "membership_badge_upload_failed", "请先创建会员阶层")
		return
	}
	if _, err := s.replaceMembershipTierBadge(w, r, config.Tiers[0]); err != nil {
		writeError(w, http.StatusBadRequest, "membership_badge_upload_failed", err.Error())
		return
	}
	updated, err := s.store.GetMembershipSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "membership_config_failed", "会员配置读取失败")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) uploadMembershipTierBadge(w http.ResponseWriter, r *http.Request) {
	tierID, _ := strconv.ParseInt(chi.URLParam(r, "tierID"), 10, 64)
	tier, err := s.store.GetMembershipTier(tierID)
	if err != nil {
		writeError(w, http.StatusNotFound, "membership_tier_not_found", "会员阶层不存在")
		return
	}
	updated, err := s.replaceMembershipTierBadge(w, r, tier)
	if err != nil {
		writeError(w, http.StatusBadRequest, "membership_badge_upload_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) replaceMembershipTierBadge(w http.ResponseWriter, r *http.Request, tier domain.MembershipTier) (domain.MembershipTier, error) {
	storedName, err := s.saveMembershipBadge(w, r)
	if err != nil {
		return domain.MembershipTier{}, err
	}
	oldURL := tier.BadgeURL
	tier.BadgeURL = "/api/v1/media/site-assets/" + storedName
	updated, err := s.store.UpdateMembershipTier(currentUser(r).ID, tier.ID, tier)
	if err != nil {
		_ = os.Remove(filepath.Join(s.uploadDir, "site-assets", storedName))
		return domain.MembershipTier{}, err
	}
	if oldName := localSiteAssetName(oldURL); oldName != "" && oldName != storedName {
		if removeErr := os.Remove(filepath.Join(s.uploadDir, "site-assets", oldName)); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			s.logger.Error("failed to remove replaced membership badge", "filename", oldName, "error", removeErr)
		}
	}
	return updated, nil
}

func (s *Server) deleteMembershipBadge(w http.ResponseWriter, r *http.Request) {
	config, err := s.store.GetMembershipSettings()
	if err != nil || len(config.Tiers) == 0 {
		writeError(w, http.StatusBadRequest, "membership_badge_delete_failed", "会员阶层不存在")
		return
	}
	if _, err := s.clearMembershipTierBadge(currentUser(r).ID, config.Tiers[0]); err != nil {
		writeError(w, http.StatusBadRequest, "membership_badge_delete_failed", err.Error())
		return
	}
	updated, err := s.store.GetMembershipSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "membership_config_failed", "会员配置读取失败")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) deleteMembershipTierBadge(w http.ResponseWriter, r *http.Request) {
	tierID, _ := strconv.ParseInt(chi.URLParam(r, "tierID"), 10, 64)
	tier, err := s.store.GetMembershipTier(tierID)
	if err != nil {
		writeError(w, http.StatusNotFound, "membership_tier_not_found", "会员阶层不存在")
		return
	}
	updated, err := s.clearMembershipTierBadge(currentUser(r).ID, tier)
	if err != nil {
		writeError(w, http.StatusBadRequest, "membership_badge_delete_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) clearMembershipTierBadge(actorID int64, tier domain.MembershipTier) (domain.MembershipTier, error) {
	oldURL := tier.BadgeURL
	tier.BadgeURL = ""
	updated, err := s.store.UpdateMembershipTier(actorID, tier.ID, tier)
	if err != nil {
		return domain.MembershipTier{}, err
	}
	removeLocalMembershipBadge(s, oldURL)
	return updated, nil
}

func removeLocalMembershipBadge(s *Server, rawURL string) {
	oldName := localSiteAssetName(rawURL)
	if oldName == "" {
		return
	}
	if removeErr := os.Remove(filepath.Join(s.uploadDir, "site-assets", oldName)); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		s.logger.Error("failed to remove membership badge", "filename", oldName, "error", removeErr)
	}
}

func (s *Server) saveMembershipBadge(w http.ResponseWriter, r *http.Request) (string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMembershipBadgeUpload+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		return "", errors.New("会员徽标文件过大或格式无效")
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		return "", errors.New("请选择会员徽标图片")
	}
	defer file.Close()
	extension := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	if extension != ".png" && extension != ".jpg" && extension != ".jpeg" {
		return "", errors.New("会员徽标仅支持 PNG 和 JPG")
	}
	firstBytes := make([]byte, 512)
	readCount, readErr := io.ReadFull(file, firstBytes)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return "", errors.New("会员徽标读取失败")
	}
	firstBytes = firstBytes[:readCount]
	mimeType := http.DetectContentType(firstBytes)
	if (extension == ".png" && mimeType != "image/png") || ((extension == ".jpg" || extension == ".jpeg") && mimeType != "image/jpeg") {
		return "", errors.New("图片内容与文件扩展名不匹配")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", errors.New("会员徽标无法重新读取")
	}
	config, _, err := image.DecodeConfig(io.LimitReader(file, maxMembershipBadgeUpload+1))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 2048 || config.Height > 2048 {
		return "", errors.New("会员徽标尺寸无效或超过 2048 × 2048")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", errors.New("会员徽标无法重新读取")
	}
	storedName, err := randomStoredName(extension)
	if err != nil {
		return "", errors.New("会员徽标文件名生成失败")
	}
	assetDir := filepath.Join(s.uploadDir, "site-assets")
	if err := os.MkdirAll(assetDir, 0750); err != nil {
		return "", errors.New("会员徽标目录创建失败")
	}
	targetPath := filepath.Join(assetDir, storedName)
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		return "", errors.New("会员徽标保存失败")
	}
	written, copyErr := io.Copy(target, io.LimitReader(file, maxMembershipBadgeUpload+1))
	syncErr := target.Sync()
	closeErr := target.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil || written == 0 || written > maxMembershipBadgeUpload {
		_ = os.Remove(targetPath)
		return "", errors.New("会员徽标为空、过大或保存失败")
	}
	return storedName, nil
}
