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

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/background"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateskills"
	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpapi"
	"github.com/lalithlochan/applyforge/apps/api/internal/preferences"
	"github.com/lalithlochan/applyforge/apps/api/internal/profile"
	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
	"github.com/lalithlochan/applyforge/apps/api/internal/skills"
	"github.com/lalithlochan/applyforge/apps/api/internal/storage"
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

	storageClient, err := storage.New(ctx, storage.Config{
		Endpoint:  getenv("S3_ENDPOINT", "localhost:9000"),
		Bucket:    getenv("S3_BUCKET", "applyforge-dev"),
		AccessKey: getenv("S3_ACCESS_KEY", "applyforge"),
		SecretKey: getenv("S3_SECRET_KEY", "applyforge123"),
		UseSSL:    getenv("S3_USE_SSL", "false") == "true",
	})
	if err != nil {
		return err
	}

	aiWorkerClient := aiclient.New(getenv("AI_WORKER_URL", "http://localhost:8000"))

	skillsNormalizer, err := skills.NewNormalizer(ctx, db)
	if err != nil {
		return err
	}

	resumeRepo := resume.NewRepository(db)
	candidateSkillsRepo := candidateskills.NewRepository(db)
	jobQueue := background.NewQueue(db)
	resumeHandlers := resume.NewHandlers(resumeRepo, storageClient, jobQueue)

	resumeParseWorker := resume.NewParseWorker(resumeRepo, candidateSkillsRepo, skillsNormalizer, storageClient, aiWorkerClient)
	worker := background.NewWorker(jobQueue, "api-inprocess-worker")
	worker.Register(resume.JobTypeParse, resumeParseWorker.Handle)

	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	go worker.Run(workerCtx, 2*time.Second)

	router := httpapi.NewRouter(httpapi.Config{
		DB:          db,
		WebBaseURL:  webBaseURL,
		RequireAuth: auth.RequireAuth(authService),
		Auth:        authHandlers,
		Authed:      []httpapi.Mounter{profileHandlers, preferencesHandlers, resumeHandlers},
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
