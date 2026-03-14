package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureDemoContent(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		INSERT INTO content(id, external_ref, title, price_stars, active)
		VALUES ($1, $2, $3, $4, TRUE)
		ON CONFLICT (id) DO NOTHING;
	`, "content-demo-1", []byte("provider:video:demo1"), "Demo Streaming Content", 25)
	return err
}
