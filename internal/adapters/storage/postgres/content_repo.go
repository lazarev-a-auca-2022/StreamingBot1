package postgres

import (
	"context"
	"streamingbot/internal/domain/content"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ContentRepo struct {
	db *pgxpool.Pool
}

func NewContentRepo(db *pgxpool.Pool) *ContentRepo {
	return &ContentRepo{db: db}
}

func (r *ContentRepo) GetByID(ctx context.Context, id string) (*content.Content, error) {
	var c content.Content
	err := r.db.QueryRow(ctx, `SELECT id, external_ref, title, price_stars, active FROM content WHERE id=$1`, id).
		Scan(&c.ID, &c.ExternalRef, &c.Title, &c.PriceStars, &c.Active)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *ContentRepo) ListActive(ctx context.Context) ([]content.Content, error) {
	rows, err := r.db.Query(ctx, `SELECT id, external_ref, title, price_stars, active FROM content WHERE active=TRUE ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []content.Content
	for rows.Next() {
		var c content.Content
		if err := rows.Scan(&c.ID, &c.ExternalRef, &c.Title, &c.PriceStars, &c.Active); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *ContentRepo) Seed(c content.Content) {
	_, _ = r.db.Exec(context.Background(), `
		INSERT INTO content(id, external_ref, title, price_stars, active)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (id) DO NOTHING
	`, c.ID, c.ExternalRef, c.Title, c.PriceStars, c.Active)
}
