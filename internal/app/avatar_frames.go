package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"roblox-community/internal/domain"
	"roblox-community/internal/store"
)

func (s *Server) listAvatarFrames(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListAvatarFrames(currentUser(r).ID, false)
	if err != nil {
		s.logger.Error("list avatar frames failed", "user_id", currentUser(r).ID, "error", err)
		writeError(w, http.StatusInternalServerError, "avatar_frames_failed", "头像框市场加载失败")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) purchaseAvatarFrame(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "frameID"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "avatar_frame_invalid", "头像框无效")
		return
	}
	item, err := s.store.PurchaseAvatarFrame(currentUser(r).ID, id)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrAvatarFrameUnavailable), errors.Is(err, sql.ErrNoRows):
			writeError(w, http.StatusNotFound, "avatar_frame_unavailable", "头像框不存在或已下架")
		case errors.Is(err, store.ErrAvatarFrameForbidden):
			writeError(w, http.StatusForbidden, "avatar_frame_forbidden", err.Error())
		case err.Error() == "钱包余额不足":
			writeError(w, http.StatusBadRequest, "wallet_insufficient", err.Error())
		default:
			s.logger.Error("purchase avatar frame failed", "user_id", currentUser(r).ID, "frame_id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "avatar_frame_purchase_failed", "头像框购买失败，请稍后重试")
		}
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) equipAvatarFrame(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FrameID int64 `json:"frame_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.FrameID < 0 {
		writeError(w, http.StatusBadRequest, "avatar_frame_invalid", "头像框无效")
		return
	}
	item, err := s.store.EquipAvatarFrame(currentUser(r).ID, input.FrameID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrAvatarFrameUnavailable), errors.Is(err, sql.ErrNoRows):
			writeError(w, http.StatusNotFound, "avatar_frame_unavailable", "头像框不存在或已下架")
		case errors.Is(err, store.ErrAvatarFrameForbidden):
			writeError(w, http.StatusForbidden, "avatar_frame_forbidden", err.Error())
		case errors.Is(err, store.ErrAvatarFrameNotOwned):
			writeError(w, http.StatusBadRequest, "avatar_frame_not_owned", err.Error())
		default:
			s.logger.Error("equip avatar frame failed", "user_id", currentUser(r).ID, "frame_id", input.FrameID, "error", err)
			writeError(w, http.StatusInternalServerError, "avatar_frame_equip_failed", "头像框装备失败，请稍后重试")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"avatar_frame": item})
}

func (s *Server) listAdminAvatarFrames(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListAvatarFrames(0, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "avatar_frames_failed", "头像框列表加载失败")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createAdminAvatarFrame(w http.ResponseWriter, r *http.Request) {
	var input domain.AvatarFrame
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.CreateAvatarFrame(currentUser(r).ID, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "avatar_frame_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) updateAdminAvatarFrame(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "frameID"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "avatar_frame_invalid", "头像框无效")
		return
	}
	var input domain.AvatarFrame
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.UpdateAvatarFrame(currentUser(r).ID, id, input)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "avatar_frame_not_found", "头像框不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "avatar_frame_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deleteAdminAvatarFrame(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "frameID"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "avatar_frame_invalid", "头像框无效")
		return
	}
	if err := s.store.DisableAvatarFrame(currentUser(r).ID, id); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "avatar_frame_not_found", "头像框不存在")
		return
	} else if err != nil {
		writeError(w, http.StatusBadRequest, "avatar_frame_disable_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"disabled": true})
}
