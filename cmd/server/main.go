package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"roblox-community/internal/app"
	"roblox-community/internal/contentmoderation"
	"roblox-community/internal/store"
)

func main() {
	driver := getenv("ROBLOX_DB_DRIVER", "mysql")
	dsn := os.Getenv("ROBLOX_MYSQL_DSN")
	masterKey := os.Getenv("ROBLOX_MASTER_KEY")
	if masterKey == "" {
		panic("ROBLOX_MASTER_KEY is required")
	}

	data, err := store.Open(driver, dsn, masterKey)
	if err != nil {
		panic(err)
	}
	defer data.Close()
	if err := data.EnsureDefaults(os.Getenv("ROBLOX_ADMIN_EMAIL"), os.Getenv("ROBLOX_ADMIN_PASSWORD")); err != nil {
		panic(err)
	}

	staticDir := getenv("ROBLOX_STATIC_DIR", filepath.Join("frontend", "dist"))
	uploadDir := getenv("ROBLOX_UPLOAD_DIR", filepath.Join("data", "uploads"))
	if err := os.MkdirAll(uploadDir, 0750); err != nil {
		panic(err)
	}
	publicURL := os.Getenv("ROBLOX_PUBLIC_URL")
	moderator, err := contentmoderation.FromEnvService()
	if err != nil {
		panic(err)
	}
	server := app.NewWithModeration(data, staticDir, uploadDir, publicURL, slog.Default(), moderator)
	addr := getenv("ROBLOX_ADDR", ":8088")
	slog.Info("roblox community listening", "addr", addr, "db_driver", driver, "content_moderation_enabled", moderator.Enabled())
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- httpServer.ListenAndServe() }()
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			panic(err)
		}
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
