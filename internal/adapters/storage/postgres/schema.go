package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, db *pgxpool.Pool) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT PRIMARY KEY,
			username TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			banned BOOLEAN NOT NULL DEFAULT FALSE
		);`,
		`CREATE TABLE IF NOT EXISTS content (
			id TEXT PRIMARY KEY,
			external_ref BYTEA NOT NULL,
			title TEXT NOT NULL,
			price_stars INTEGER NOT NULL CHECK (price_stars > 0),
			active BOOLEAN NOT NULL DEFAULT TRUE
		);`,
		`CREATE TABLE IF NOT EXISTS purchases (
			id TEXT PRIMARY KEY,
			user_id BIGINT NOT NULL,
			content_id TEXT NOT NULL,
			status TEXT NOT NULL,
			telegram_payload TEXT NOT NULL UNIQUE,
			telegram_charge_id TEXT UNIQUE,
			stars_amount INTEGER NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			paid_at TIMESTAMPTZ,
			issue_link_attempts INTEGER NOT NULL DEFAULT 0,
			last_issue_link_error TEXT,
			last_issue_link_at TIMESTAMPTZ,
			review_requested_at TIMESTAMPTZ
		);`,
		`CREATE TABLE IF NOT EXISTS access_grants (
			id TEXT PRIMARY KEY,
			purchase_id TEXT NOT NULL UNIQUE,
			user_id BIGINT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			issued_at TIMESTAMPTZ NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ
		);`,
		`CREATE TABLE IF NOT EXISTS reviews (
			id TEXT PRIMARY KEY,
			user_id BIGINT NOT NULL,
			purchase_id TEXT NOT NULL UNIQUE,
			rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
			text TEXT,
			published BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS payment_events (
			id BIGSERIAL PRIMARY KEY,
			charge_id TEXT NOT NULL UNIQUE,
			amount_stars INTEGER NOT NULL,
			invoice_payload TEXT NOT NULL,
			raw_payload BYTEA NOT NULL,
			occurred_at TIMESTAMPTZ NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS idempotency_keys (
			event_id TEXT PRIMARY KEY,
			processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS outbox_events (
			id TEXT PRIMARY KEY,
			event_type TEXT NOT NULL,
			purchase_id TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			published BOOLEAN NOT NULL DEFAULT FALSE
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(ctx, q); err != nil {
			return fmt.Errorf("ensure schema: %w", err)
		}
	}
	return nil
}
