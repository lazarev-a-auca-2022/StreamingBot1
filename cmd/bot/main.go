package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"streamingbot/internal/adapters/httpapi"
	"streamingbot/internal/adapters/storage/postgres"
	"streamingbot/internal/adapters/streaming"
	"streamingbot/internal/adapters/telegram"
	"streamingbot/internal/app/confirm_payment"
	"streamingbot/internal/app/issue_access"
	"streamingbot/internal/app/start_purchase"
	"streamingbot/internal/app/submit_review"
	"streamingbot/internal/app/use_access"
	"streamingbot/internal/domain/content"
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

	contentRepo := postgres.NewContentRepo()
	purchaseRepo := postgres.NewPurchaseRepo()
	accessRepo := postgres.NewAccessRepo()
	reviewRepo := postgres.NewReviewRepo()
	_ = reviewRepo
	idempotencyRepo := postgres.NewIdempotencyRepo()
	eventLogRepo := postgres.NewEventLogRepo()
	outboxRepo := postgres.NewOutboxRepo()

	seedContent(contentRepo)

	startPurchaseUC := start_purchase.Handler{
		Purchases: purchaseRepo,
		Contents:  contentRepo,
		IDs:       idgen.NewService(),
	}
	issueAccessUC := issue_access.Handler{
		Purchases:  purchaseRepo,
		Contents:   contentRepo,
		Grants:     accessRepo,
		Provider:   streaming.NewClient(cfg.StreamingAPIURL, cfg.StreamingAPIKey),
		Tokens:     crypto.NewTokenService(),
		Sender:     telegram.NewSender(),
		TTL:        time.Duration(cfg.AccessLinkTTLMinutes) * time.Minute,
		MaxRetries: 3,
	}
	confirmPaymentUC := confirm_payment.Handler{
		Purchases:   purchaseRepo,
		Idempotency: idempotencyRepo,
		EventLog:    eventLogRepo,
		Outbox:      outboxRepo,
	}
	useAccessUC := use_access.Handler{Grants: accessRepo}
	submitReviewUC := submit_review.Handler{Purchases: purchaseRepo, Reviews: reviewRepo}

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
		if err := httpapi.StartServer(ctx, ":8080", api.Handler()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
			stop()
		}
	}()

	lg.Info("streamingbot server listening on :8080")
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

func seedContent(repo *postgres.ContentRepo) {
	repo.Seed(content.Content{
		ID:          "content-demo-1",
		ExternalRef: []byte("provider:video:demo1"),
		Title:       "Demo Streaming Content",
		PriceStars:  25,
		Active:      true,
	})
}
