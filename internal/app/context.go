package app

import (
	"context"
	"net/http"

	"roblox-community/internal/domain"
)

func withUser(ctx context.Context, user domain.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func currentUser(r *http.Request) domain.User {
	user, _ := r.Context().Value(userKey).(domain.User)
	return user
}
