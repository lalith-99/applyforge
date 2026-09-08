// Command api runs the ApplyForge Go API server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/account"
	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/airank"
	"github.com/lalithlochan/applyforge/apps/api/internal/aiusage"
	"github.com/lalithlochan/applyforge/apps/api/internal/analytics"
	"github.com/lalithlochan/applyforge/apps/api/internal/applications"
	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/background"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateprofile"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateskills"
	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpapi"
	"github.com/lalithlochan/applyforge/apps/api/internal/immigration"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobrecommendations"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobrequirements"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/learning"
	"github.com/lalithlochan/applyforge/apps/api/internal/matching"
	"github.com/lalithlochan/applyforge/apps/api/internal/preferences"
	"github.com/lalithlochan/applyforge/apps/api/internal/profile"
	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
	"github.com/lalithlochan/applyforge/apps/api/internal/resumeversion"
	"github.com/lalithlochan/applyforge/apps/api/internal/scheduler"
	"github.com/lalithlochan/applyforge/apps/api/internal/skills"
	"github.com/lalithlochan/applyforge/apps/api/internal/storage"
	"github.com/lalithlochan/applyforge/apps/api/internal/tailoring"
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

	addr := strings.TrimSpace(os.Getenv("API_ADDR"))
	if addr == "" {
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			addr = ":" + port
		} else {
			addr = ":8080"
		}
	}
	dsn := getenv("DATABASE_URL", "postgres://applyforge:applyforge@localhost:5432/applyforge?sslmode=disable")
	webBaseURL := getenv("WEB_BASE_URL", "http://localhost:3000")
	environment := getenv("ENVIRONMENT", "development")
	if err := validateProductionConfig(environment); err != nil {
		return err
	}

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
	authActions := auth.NewActionService(db, auth.NewMailerFromEnv(), webBaseURL)
	cookieSameSite, err := authCookieSameSite(environment)
	if err != nil {
		return err
	}
	authHandlers := auth.NewHandlers(authService, webBaseURL, environment == "production").
		WithCookieSameSite(cookieSameSite).
		WithActionService(authActions)

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
	aiUsageRepo := aiusage.NewRepository(db)
	aiWorkerClient.SetUsageRecorder(func(ctx context.Context, operation string, latencyMS int64, status string, errMsg *string) {
		aiUsageRepo.RecordAsync(ctx, aiusage.Entry{Operation: operation, Status: status, LatencyMS: latencyMS, ErrorMessage: errMsg})
	})
	aiWorkerClient.SetDetailedUsageRecorder(func(ctx context.Context, usage aiclient.DetailedUsage) {
		aiUsageRepo.RecordAsync(ctx, aiusage.Entry{
			Operation:        usage.Operation,
			Status:           usage.Status,
			LatencyMS:        usage.LatencyMS,
			ErrorMessage:     usage.ErrorMessage,
			Provider:         usage.Provider,
			Model:            usage.Model,
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
			EstimatedCostUSD: usage.EstimatedCostUSD,
		})
	})

	skillsNormalizer, err := skills.NewNormalizer(ctx, db)
	if err != nil {
		return err
	}

	resumeRepo := resume.NewRepository(db)
	candidateSkillsRepo := candidateskills.NewRepository(db)
	jobQueue := background.NewQueue(db)
	resumeHandlers := resume.NewHandlers(resumeRepo, storageClient, jobQueue)

	resumeParseWorker := resume.NewParseWorker(resumeRepo, candidateSkillsRepo, skillsNormalizer, storageClient, aiWorkerClient)

	candidateProfileRepo := candidateprofile.NewRepository(db)
	candidateProfileService := candidateprofile.NewService(candidateProfileRepo, resumeRepo, candidateSkillsRepo, preferencesRepo, profileRepo, aiWorkerClient)
	candidateProfileWorker := candidateprofile.NewBuildWorker(candidateProfileService, candidateProfileRepo, aiWorkerClient)
	resumeParseWorker.SetOnParsed(func(ctx context.Context, userID uuid.UUID) {
		if err := jobQueue.Enqueue(ctx, candidateprofile.JobTypeBuild, candidateprofile.BuildPayload{UserID: userID.String()}, 3); err != nil {
			slog.Error("enqueue build_candidate_profile failed", "user_id", userID, "error", err)
		}
	})

	jobsRepo := jobs.NewRepository(db)
	if discovered, err := jobsRepo.BackfillDiscoveredCompanySources(ctx); err != nil {
		return fmt.Errorf("backfill discovered company job sources: %w", err)
	} else if discovered > 0 {
		slog.Info("restored direct ATS sources from existing catalog", "sources", discovered)
	}
	// Direct ATS sources are enabled dynamically when ApplyForge learns a
	// company's public Greenhouse/Lever/Ashby/SmartRecruiters/Workable endpoint.
	// Broad providers remain discovery/gap sources rather than replacing these
	// authoritative employer feeds.

	brightDataEnabled := strings.EqualFold(getenv("BRIGHTDATA_ENABLED", "false"), "true")
	brightDataShards := []string{"us-single-user-core-24h"}
	if raw := strings.TrimSpace(os.Getenv("BRIGHTDATA_ACTIVE_SHARDS")); raw != "" {
		brightDataShards = strings.Split(raw, ",")
	}
	brightDataPollMinutes := 1440
	if raw := strings.TrimSpace(os.Getenv("BRIGHTDATA_POLL_INTERVAL_MINUTES")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 15 || parsed > 1440 {
			return fmt.Errorf("BRIGHTDATA_POLL_INTERVAL_MINUTES must be an integer between 15 and 1440")
		}
		brightDataPollMinutes = parsed
	}

	var brightDataConfig jobs.BrightDataConfig
	if brightDataEnabled {
		var err error
		brightDataConfig, err = jobs.BrightDataConfigFromEnv()
		if err != nil {
			return fmt.Errorf("Bright Data ingestion enabled but configuration is invalid: %w", err)
		}
	}
	if err := jobsRepo.ConfigureSourceShards(ctx, "BRIGHTDATA", brightDataEnabled, brightDataShards, brightDataPollMinutes); err != nil {
		return fmt.Errorf("configure Bright Data job sources: %w", err)
	}
	if brightDataEnabled {
		slog.Info("Bright Data discovery configured",
			"shards", brightDataShards,
			"records_limit_per_poll", brightDataConfig.RecordsLimit,
			"poll_interval_minutes", brightDataPollMinutes,
		)
	}

	googleJobsEnabled := strings.EqualFold(getenv("SERPAPI_GOOGLE_JOBS_ENABLED", "false"), "true")
	if googleJobsEnabled {
		if _, err := jobs.SerpAPIGoogleJobsConfigFromEnv(); err != nil {
			return fmt.Errorf("Google Jobs discovery enabled but configuration is invalid: %w", err)
		}
	}
	if err := jobsRepo.SetSourceTypeEnabled(ctx, "SERPAPI_GOOGLE_JOBS", googleJobsEnabled); err != nil {
		return fmt.Errorf("configure Google Jobs discovery sources: %w", err)
	}

	ingestionService := jobs.NewIngestionService(jobsRepo, jobQueue)
	jobRequirementsRepo := jobrequirements.NewRepository(db)
	jobRequirementsService := jobrequirements.NewService(jobRequirementsRepo, aiWorkerClient).WithUsageTracking(aiUsageRepo)
	adminSyncToken := os.Getenv(strings.Join([]string{"ADMIN", "SYNC", "TOKEN"}, "_"))
	jobsHandlers := jobs.NewHandlers(jobsRepo, ingestionService, jobRequirementsService).
		WithPreferences(preferencesRepo).
		WithAdminSyncToken(adminSyncToken)

	immigrationRepo := immigration.NewRepository(db)
	immigrationHandlers := immigration.NewHandlers(immigrationRepo, adminSyncToken)

	var (
		companySourceDiscoveryWorker    *jobs.CompanySourceDiscoveryWorker
		companySourceDiscoveryScheduler *jobs.CompanySourceDiscoveryScheduler
	)
	sourceDiscoveryEnabled := strings.EqualFold(getenv("DATAFORSEO_SOURCE_DISCOVERY_ENABLED", "false"), "true")
	if sourceDiscoveryEnabled {
		resolverConfig, err := jobs.DataForSEOCompanySourceConfigFromEnv()
		if err != nil {
			return fmt.Errorf("DataForSEO source discovery enabled but configuration is invalid: %w", err)
		}

		batchSize := 500
		if raw := strings.TrimSpace(os.Getenv("DATAFORSEO_SOURCE_DISCOVERY_BATCH_SIZE")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 1000 {
				return errors.New("DATAFORSEO_SOURCE_DISCOVERY_BATCH_SIZE must be between 1 and 1000")
			}
			batchSize = parsed
		}
		intervalMinutes := 15
		if raw := strings.TrimSpace(os.Getenv("DATAFORSEO_SOURCE_DISCOVERY_INTERVAL_MINUTES")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 1440 {
				return errors.New("DATAFORSEO_SOURCE_DISCOVERY_INTERVAL_MINUTES must be between 1 and 1440")
			}
			intervalMinutes = parsed
		}
		maxPerDay := 10000
		if raw := strings.TrimSpace(os.Getenv("DATAFORSEO_SOURCE_DISCOVERY_MAX_REQUESTS_PER_DAY")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 {
				return errors.New("DATAFORSEO_SOURCE_DISCOVERY_MAX_REQUESTS_PER_DAY must be positive")
			}
			maxPerDay = parsed
		}
		maxPerMonth := 10000
		if raw := strings.TrimSpace(os.Getenv("DATAFORSEO_SOURCE_DISCOVERY_MAX_REQUESTS_PER_MONTH")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 {
				return errors.New("DATAFORSEO_SOURCE_DISCOVERY_MAX_REQUESTS_PER_MONTH must be positive")
			}
			maxPerMonth = parsed
		}
		estimatedCostUSD := 0.002
		if raw := strings.TrimSpace(os.Getenv("DATAFORSEO_SOURCE_DISCOVERY_ESTIMATED_USD_PER_REQUEST")); raw != "" {
			parsed, err := strconv.ParseFloat(raw, 64)
			if err != nil || parsed < 0 {
				return errors.New("DATAFORSEO_SOURCE_DISCOVERY_ESTIMATED_USD_PER_REQUEST must be non-negative")
			}
			estimatedCostUSD = parsed
		}

		budget := jobs.ProviderRequestBudget{
			MaxPerDay:        maxPerDay,
			MaxPerMonth:      maxPerMonth,
			EstimatedCostUSD: estimatedCostUSD,
		}
		resolver := jobs.NewDataForSEOCompanySourceResolver(resolverConfig)
		companySourceDiscoveryWorker = jobs.NewCompanySourceDiscoveryWorker(jobsRepo, resolver, budget)
		companySourceDiscoveryScheduler = jobs.NewCompanySourceDiscoveryScheduler(
			jobsRepo,
			jobQueue,
			batchSize,
			2*time.Hour,
		)
		slog.Info("DataForSEO company source discovery configured",
			"batch_size", batchSize,
			"interval_minutes", intervalMinutes,
			"max_requests_per_day", maxPerDay,
			"max_requests_per_month", maxPerMonth,
		)
		_ = intervalMinutes
	}

	syncSourceWorker := jobs.NewSyncSourceWorker(jobsRepo, ingestionService)
	roleWorker := jobs.NewClassifyRoleWorker(jobsRepo, aiWorkerClient, jobQueue)
	catalogBackfillWorker := jobs.NewCatalogBackfillWorker(jobsRepo, jobQueue)
	enrichWorker := jobs.NewEnrichWorker(jobsRepo, jobRequirementsService)
	embedWorker := jobs.NewEmbedWorker(jobsRepo, aiWorkerClient)

	matchingRepo := matching.NewRepository(db)
	matchingService := matching.NewService(matchingRepo, candidateSkillsRepo, jobsRepo, jobRequirementsService, preferencesRepo, profileRepo, candidateProfileRepo).
		WithImmigrationEvidence(immigrationRepo)
	matchingHandlers := matching.NewHandlers(matchingService)

	airankService := airank.NewService(aiWorkerClient)
	jobRecommendationsRepo := jobrecommendations.NewRepository(db)
	jobRecommendationsHandlers := jobrecommendations.NewHandlers(jobRecommendationsRepo)
	jobRecommendationsWorker := jobrecommendations.NewComputeWorker(matchingService, airankService, candidateProfileRepo, jobRecommendationsRepo)
	candidateProfileWorker.SetOnBuilt(func(ctx context.Context, userID uuid.UUID) {
		if err := jobQueue.Enqueue(ctx, jobrecommendations.JobTypeCompute, jobrecommendations.ComputePayload{UserID: userID.String()}, 3); err != nil {
			slog.Error("enqueue compute_recommendations failed", "user_id", userID, "error", err)
		}
	})
	preferencesHandlers.SetOnChanged(func(ctx context.Context, userID uuid.UUID) {
		if err := jobQueue.Enqueue(ctx, jobrecommendations.JobTypeCompute, jobrecommendations.ComputePayload{UserID: userID.String()}, 3); err != nil {
			slog.Error("enqueue compute_recommendations failed", "user_id", userID, "error", err)
		}
	})
	profileHandlers.SetOnChanged(func(ctx context.Context, userID uuid.UUID) {
		if err := jobQueue.Enqueue(ctx, jobrecommendations.JobTypeCompute, jobrecommendations.ComputePayload{UserID: userID.String()}, 3); err != nil {
			slog.Error("enqueue compute_recommendations failed", "user_id", userID, "error", err)
		}
	})

	tailoringRepo := tailoring.NewRepository(db)
	tailoringService := tailoring.NewService(tailoringRepo, resumeRepo, candidateSkillsRepo, jobsRepo, jobRequirementsService, matchingRepo, aiWorkerClient)
	tailoringHandlers := tailoring.NewHandlers(tailoringService, tailoringRepo, jobQueue)
	tailoringWorker := tailoring.NewWorker(tailoringService)

	// Multiple worker goroutines claim from the shared queue concurrently
	// (SELECT ... FOR UPDATE SKIP LOCKED makes this safe), so slow/rate
	// -limited providers or AI calls don't serialize every other job.
	workerCount := 5
	if v := os.Getenv("BACKGROUND_WORKER_COUNT"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			workerCount = parsed
		}
	}
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	for i := 0; i < workerCount; i++ {
		w := background.NewWorker(jobQueue, fmt.Sprintf("api-inprocess-worker-%d", i))
		w.Register(resume.JobTypeParse, resumeParseWorker.Handle)
		w.Register(jobs.JobTypeSyncSource, syncSourceWorker.Handle)
		if companySourceDiscoveryWorker != nil {
			w.Register(jobs.JobTypeResolveCompanySource, companySourceDiscoveryWorker.Handle)
		}
		w.Register(jobs.JobTypeClassifyRole, roleWorker.Handle)
		w.Register(jobs.JobTypeCatalogBackfill, catalogBackfillWorker.Handle)
		w.Register(jobs.JobTypeEnrich, enrichWorker.Handle)
		w.Register(jobs.JobTypeEmbed, embedWorker.Handle)
		w.Register(candidateprofile.JobTypeBuild, candidateProfileWorker.Handle)
		w.Register(jobrecommendations.JobTypeCompute, jobRecommendationsWorker.Handle)
		w.Register(tailoring.JobTypeProcess, tailoringWorker.Handle)
		go w.Run(workerCtx, 2*time.Second)
	}

	pollMinutes := 60
	if v := os.Getenv("JOB_POLL_INTERVAL_MINUTES"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			pollMinutes = parsed
		}
	}
	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	defer stopScheduler()
	go scheduler.Run(schedulerCtx, ingestionService, time.Duration(pollMinutes)*time.Minute)

	if companySourceDiscoveryScheduler != nil {
		intervalMinutes := 15
		if raw := strings.TrimSpace(os.Getenv("DATAFORSEO_SOURCE_DISCOVERY_INTERVAL_MINUTES")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1 && parsed <= 1440 {
				intervalMinutes = parsed
			}
		}
		go companySourceDiscoveryScheduler.Run(schedulerCtx, time.Duration(intervalMinutes)*time.Minute)
	}

	recommendationRefreshMinutes := 60
	if v := os.Getenv("RECOMMENDATION_REFRESH_INTERVAL_MINUTES"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			recommendationRefreshMinutes = parsed
		}
	}
	recommendationRefreshCtx, stopRecommendationRefresh := context.WithCancel(context.Background())
	defer stopRecommendationRefresh()
	go func() {
		ticker := time.NewTicker(time.Duration(recommendationRefreshMinutes) * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-recommendationRefreshCtx.Done():
				return
			case <-ticker.C:
				if err := jobrecommendations.EnqueueForActiveUsers(recommendationRefreshCtx, jobQueue, candidateProfileRepo); err != nil {
					slog.Error("periodic recommendation refresh failed", "error", err)
				}
			}
		}
	}()

	learningRepo := learning.NewRepository(db)
	learningService := learning.NewService(learningRepo, aiWorkerClient, candidateSkillsRepo, matchingRepo, matchingService)
	learningHandlers := learning.NewHandlers(learningService)

	resumeVersionRepo := resumeversion.NewRepository(db)
	resumeVersionService := resumeversion.NewService(resumeVersionRepo, resumeRepo, tailoringRepo, aiWorkerClient, storageClient, matchingService)
	resumeVersionHandlers := resumeversion.NewHandlers(resumeVersionService, resumeVersionRepo, resumeRepo)

	applicationsRepo := applications.NewRepository(db)
	applicationsService := applications.NewService(applicationsRepo)
	applicationsHandlers := applications.NewHandlers(applicationsService, applicationsRepo)

	analyticsRepo := analytics.NewRepository(db)
	analyticsService := analytics.NewService(analyticsRepo, applicationsRepo)
	analyticsHandlers := analytics.NewHandlers(analyticsService)

	accountService := account.NewService(userRepo, resumeRepo, resumeVersionRepo, storageClient)
	accountHandlers := account.NewHandlers(accountService, environment == "production")

	requireAuthMiddleware := auth.RequireAuth(authService)
	if strings.EqualFold(getenv("REQUIRE_EMAIL_VERIFICATION", "false"), "true") {
		baseRequireAuth := requireAuthMiddleware
		requireAuthMiddleware = func(next http.Handler) http.Handler {
			return baseRequireAuth(auth.RequireVerifiedEmail(next))
		}
	}

	router := httpapi.NewRouter(httpapi.Config{
		DB:          db,
		WebBaseURL:  webBaseURL,
		RequireAuth: requireAuthMiddleware,
		Auth:        authHandlers,
		Admin: []httpapi.Mounter{
			immigrationHandlers,
		},

		Authed: []httpapi.Mounter{
			profileHandlers,
			preferencesHandlers,
			resumeHandlers,
			jobsHandlers,
			matchingHandlers,
			tailoringHandlers,
			learningHandlers,
			resumeVersionHandlers,
			applicationsHandlers,
			analyticsHandlers,
			accountHandlers,
			jobRecommendationsHandlers,
		},
		RateLimitStore: httpapi.NewPostgresRateLimitStore(db),
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
