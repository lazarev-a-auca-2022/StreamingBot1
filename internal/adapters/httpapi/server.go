package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"streamingbot/internal/app/confirm_payment"
	"streamingbot/internal/app/start_purchase"
	"streamingbot/internal/app/submit_review"
	"streamingbot/internal/app/use_access"
	"streamingbot/internal/domain/content"
	"streamingbot/internal/domain/payment"
	"strings"
	"time"
)

type Server struct {
	Catalog        content.Repository
	StartPurchase  start_purchase.Handler
	ConfirmPayment confirm_payment.Handler
	UseAccess      use_access.Handler
	SubmitReview   submit_review.Handler
	WebhookSecret  string
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/catalog", s.handleCatalog)
	mux.HandleFunc("/purchase/start", s.handleStartPurchase)
	mux.HandleFunc("/webhook/telegram/successful_payment", s.handleSuccessfulPayment)
	mux.HandleFunc("/access/use", s.handleUseAccess)
	mux.HandleFunc("/review/submit", s.handleSubmitReview)
	return mux
}

func (s Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	items, err := s.Catalog.ListActive(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s Server) handleStartPurchase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var cmd start_purchase.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.StartPurchase.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s Server) handleSuccessfulPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	if s.WebhookSecret != "" && r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != s.WebhookSecret {
		writeError(w, http.StatusUnauthorized, errors.New("invalid webhook secret"))
		return
	}

	var body struct {
		ChargeID       string `json:"charge_id"`
		AmountStars    int    `json:"amount_stars"`
		InvoicePayload string `json:"invoice_payload"`
		RawPayload     string `json:"raw_payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	e := payment.Event{
		ChargeID:       strings.TrimSpace(body.ChargeID),
		AmountStars:    body.AmountStars,
		InvoicePayload: body.InvoicePayload,
		RawPayload:     []byte(body.RawPayload),
		OccurredAt:     time.Now(),
	}
	if err := s.ConfirmPayment.Handle(r.Context(), confirm_payment.Command{Event: e}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "accepted"})
}

func (s Server) handleUseAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var cmd use_access.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.UseAccess.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s Server) handleSubmitReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var cmd submit_review.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.SubmitReview.Handle(r.Context(), cmd); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "saved"})
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	b := mustJSON(v)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"error":"serialization"}`)
	}
	return b
}

func StartServer(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
