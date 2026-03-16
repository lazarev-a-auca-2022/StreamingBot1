package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"streamingbot/internal/adapters/httpapi"
	"streamingbot/internal/adapters/storage/postgres"
	"streamingbot/internal/adapters/storage/redis"
	"streamingbot/internal/adapters/streaming"
	"streamingbot/internal/adapters/telegram"
	"streamingbot/internal/app/confirm_payment"
	"streamingbot/internal/app/issue_access"
	"streamingbot/internal/app/start_purchase"
	"streamingbot/internal/app/submit_review"
	"streamingbot/internal/app/use_access"
	"streamingbot/internal/jobs"
	"streamingbot/internal/platform/config"
	"streamingbot/internal/platform/crypto"
	"streamingbot/internal/platform/idgen"
	"streamingbot/internal/platform/logger"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	lg := logger.New(cfg.LogLevel)
	lg.Info("streamingbot bootstrap started")

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	if err := postgres.EnsureSchema(ctx, db); err != nil {
		log.Fatalf("ensure schema: %v", err)
	}

	tokenCache, err := redis.NewTokenStore(cfg.RedisURL)
	if err != nil {
		log.Fatalf("open redis: %v", err)
	}
	defer func() { _ = tokenCache.Close() }()
	if err := tokenCache.Ping(ctx); err != nil {
		log.Fatalf("ping redis: %v", err)
	}

	contentRepo := postgres.NewContentRepo(db)
	purchaseRepo := postgres.NewPurchaseRepo(db)
	accessRepo := postgres.NewAccessRepo(db)
	reviewRepo := postgres.NewReviewRepo(db)
	idempotencyRepo := postgres.NewIdempotencyRepo(db)
	eventLogRepo := postgres.NewEventLogRepo(db)
	outboxRepo := postgres.NewOutboxRepo(db)
	bunnyClient := streaming.NewBunnyClient(cfg.BunnyLibraryID, cfg.BunnyAPIKey, cfg.BunnyAPIBaseURL, cfg.BunnyEmbedBaseURL, cfg.BunnyTokenAuthKey)

	if cfg.BunnyLibraryID != "" && cfg.BunnyAPIKey != "" {
		synced, err := streaming.SyncLibraryContent(ctx, bunnyClient, contentRepo, cfg.BunnyDefaultPrice)
		if err != nil {
			if cfg.Environment == "production" || cfg.Environment == "prod" {
				log.Fatalf("initial bunny sync failed: %v", err)
			}
			log.Printf("initial bunny sync failed: %v", err)
		} else {
			log.Printf("initial bunny sync complete: %d items", synced)
		}
	} else {
		if err := postgres.EnsureDemoContent(ctx, db); err != nil {
			log.Fatalf("seed content: %v", err)
		}
	}

	startPurchaseUC := start_purchase.Handler{
		Purchases: purchaseRepo,
		Contents:  contentRepo,
		IDs:       idgen.NewService(),
	}
	confirmPaymentUC := confirm_payment.Handler{
		Purchases:   purchaseRepo,
		Idempotency: idempotencyRepo,
		EventLog:    eventLogRepo,
		Outbox:      outboxRepo,
	}
	submitReviewUC := submit_review.Handler{Purchases: purchaseRepo, Reviews: reviewRepo}
	telegramSender := telegram.NewSender(nil)
	var tgBot *telegram.Bot
	if cfg.TelegramPolling {
		var err error
		tgBot, err = telegram.NewBot(cfg.BotToken, cfg.TelegramPollTimeout, cfg.WebhookSecret, contentRepo, startPurchaseUC, confirmPaymentUC, submitReviewUC)
		if err != nil {
			if cfg.Environment == "production" || cfg.Environment == "prod" {
				log.Fatalf("telegram bot init: %v", err)
			}
			log.Printf("telegram bot disabled (init error): %v", err)
			tgBot = nil
		} else {
			telegramSender = telegram.NewSender(tgBot.API())
		}
	}

	issueAccessUC := issue_access.Handler{
		Purchases:  purchaseRepo,
		Contents:   contentRepo,
		Grants:     accessRepo,
		Provider:   bunnyClient,
		Tokens:     crypto.NewTokenService(),
		Sender:     telegramSender,
		Cache:      tokenCache,
		TTL:        24 * time.Hour,
		MaxRetries: 3,
	}
	useAccessUC := use_access.Handler{Grants: accessRepo, Cache: tokenCache}

	if tgBot != nil {
		go func() {
			if err := tgBot.Start(ctx); err != nil {
				log.Printf("telegram bot stopped: %v", err)
			}
		}()
	}

	if cfg.BunnyLibraryID != "" && cfg.BunnyAPIKey != "" && cfg.BunnySyncIntervalSec > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(cfg.BunnySyncIntervalSec) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					synced, err := streaming.SyncLibraryContent(ctx, bunnyClient, contentRepo, cfg.BunnyDefaultPrice)
					if err != nil {
						log.Printf("bunny periodic sync failed: %v", err)
						continue
					}
					if synced > 0 {
						log.Printf("bunny periodic sync complete: %d items", synced)
					}
				}
			}
		}()
	}

	api := httpapi.Server{
		Catalog:        contentRepo,
		StartPurchase:  startPurchaseUC,
		ConfirmPayment: confirmPaymentUC,
		UseAccess:      useAccessUC,
		SubmitReview:   submitReviewUC,
		WebhookSecret:  cfg.WebhookSecret,
	}

	processor := jobs.OutboxProcessor{
		Outbox:      outboxAdapter{repo: outboxRepo},
		IssueAccess: issueAccessUC,
	}
	scheduler := jobs.NewScheduler(2 * time.Second)
	go scheduler.Start(ctx, processor.RunOnce)

	go func() {
		if err := httpapi.StartServer(ctx, cfg.HTTPAddr, api.Handler()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
			stop()
		}
	}()

	lg.Info("streamingbot server listening on " + cfg.HTTPAddr)
	<-ctx.Done()
	lg.Info("streamingbot shutdown complete")
}

type outboxAdapter struct {
	repo *postgres.OutboxRepo
}

func (o outboxAdapter) Unpublished(ctx context.Context, limit int) ([]jobs.OutboxEvent, error) {
	events, err := o.repo.Unpublished(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]jobs.OutboxEvent, 0, len(events))
	for _, e := range events {
		out = append(out, jobs.OutboxEvent{ID: e.ID, Type: e.Type, PurchaseID: e.PurchaseID})
	}
	return out, nil
}

func (o outboxAdapter) MarkPublished(ctx context.Context, id string) error {
	return o.repo.MarkPublished(ctx, id)
}
