package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"roblox-community/internal/domain"
)

type robloxAssetDetails struct {
	AssetID     int64  `json:"AssetId"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	AssetTypeID int64  `json:"AssetTypeId"`
	Creator     struct {
		Name string `json:"Name"`
	} `json:"Creator"`
}

func parseRobloxAssetID(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 || id > 9_000_000_000_000_000_000 {
		return 0, errors.New("Roblox 音乐 ID 无效")
	}
	return id, nil
}

func fetchRobloxAsset(assetID int64) (domain.RobloxMusic, error) {
	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://economy.roblox.com/v2/assets/%d/details", assetID), nil)
	if err != nil {
		return domain.RobloxMusic{}, err
	}
	request.Header.Set("User-Agent", "RobForum/1.0 Roblox music lookup")
	client := &http.Client{Timeout: 8 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return domain.RobloxMusic{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return domain.RobloxMusic{}, errors.New("Roblox 音乐不存在或暂时无法查询")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return domain.RobloxMusic{}, err
	}
	var details robloxAssetDetails
	if err := json.Unmarshal(body, &details); err != nil || details.AssetID == 0 {
		return domain.RobloxMusic{}, errors.New("Roblox 返回了无效的音乐信息")
	}
	if details.AssetTypeID != 3 {
		return domain.RobloxMusic{}, errors.New("该 ID 不是 Roblox 音频资源")
	}
	return domain.RobloxMusic{
		AssetID: details.AssetID, Name: strings.TrimSpace(details.Name), Description: strings.TrimSpace(details.Description), CreatorName: strings.TrimSpace(details.Creator.Name),
		ThumbnailURL: fmt.Sprintf("https://www.roblox.com/asset-thumbnail/image?assetId=%d&width=420&height=420&format=png", details.AssetID),
	}, nil
}

func (s *Server) lookupRobloxMusic(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		query = r.URL.Query().Get("query")
	}
	items, err := s.store.ListApprovedRobloxMusic(query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "music_failed", "音乐库加载失败")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) listMyRobloxMusic(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListRobloxMusicFavorites(currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "music_failed", "音乐收藏加载失败")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) favoriteRobloxMusic(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "assetID")
	if !ok {
		writeError(w, http.StatusBadRequest, "music_invalid", "Roblox 音乐 ID 无效")
		return
	}
	item, err := s.store.ApprovedRobloxMusic(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "music_not_found", "该音乐尚未通过审核或不存在")
		return
	}
	if err := s.store.SaveRobloxMusicFavorite(currentUser(r).ID, item); err != nil {
		writeError(w, http.StatusInternalServerError, "music_failed", "音乐收藏保存失败")
		return
	}
	item.Favorited = true
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) unfavoriteRobloxMusic(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "assetID")
	if !ok {
		writeError(w, http.StatusBadRequest, "music_invalid", "Roblox 音乐 ID 无效")
		return
	}
	if err := s.store.DeleteRobloxMusicFavorite(currentUser(r).ID, id); err != nil {
		writeError(w, http.StatusNotFound, "music_not_found", "该音乐不在收藏夹中")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"asset_id": id, "favorited": false})
}

func (s *Server) createRobloxMusicSubmission(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AssetID  int64  `json:"asset_id"`
		Name     string `json:"name"`
		ImageURL string `json:"image_url"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.CreateRobloxMusicSubmission(currentUser(r).ID, input.AssetID, input.Name, input.ImageURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "music_submission_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) listMyRobloxMusicSubmissions(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListMyRobloxMusicSubmissions(currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "music_submissions_failed", "音乐投稿记录加载失败")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) listAdminRobloxMusicSubmissions(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListAdminRobloxMusicSubmissions(r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "music_submissions_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) reviewAdminRobloxMusicSubmission(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "submissionID")
	if !ok {
		writeError(w, http.StatusBadRequest, "music_submission_invalid", "音乐投稿无效")
		return
	}
	var input struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := s.store.ReviewRobloxMusicSubmission(currentUser(r).ID, id, input.Status, input.Note)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "music_submission_not_found", "音乐投稿不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "music_submission_review_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}
