// Command api runs the ApplyForge Go API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpapi"
	"github.com/lalithlochan/applyforge/apps/api/internal/preferences"
	"github.com/lalithlochan/applyforge/apps/api/internal/profile"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := getenv("API_ADDR", ":8080")
	dsn := getenv("DATABASE_URL", "postgres://applyforge:applyforge@localhost:5432/applyforge?sslmode=disable")
	webBaseURL := getenv("WEB_BASE_URL", "http://localhost:3000")
	environment := getenv("ENVIRONMENT", "development")

	db, err := database.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	userRepo := users.NewRepository(db)
	authService := auth.NewService(db, userRepo, auth.GoogleConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
	})
	authHandlers := auth.NewHandlers(authService, webBaseURL, environment == "production")

	profileRepo := profile.NewRepository(db)
	profileHandlers := profile.NewHandlers(profileRepo)

	preferencesRepo := preferences.NewRepository(db)
	preferencesHandlers := preferences.NewHandlers(preferencesRepo)

	router := httpapi.NewRouter(httpapi.Config{
		DB:          db,
		WebBaseURL:  webBaseURL,
		RequireAuth: auth.RequireAuth(authService),
		Auth:        authHandlers,
		Profile:     profileHandlers,
		Preferences: preferencesHandlers,
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down api")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
