package app

import "net/http"

func (s *Server) myCheckin(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.GetCheckinSummary(currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "checkin_failed", "签到信息加载失败")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) createCheckin(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.Checkin(currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "checkin_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
