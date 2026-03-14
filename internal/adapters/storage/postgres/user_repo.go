package postgres

import (
	"context"
	"streamingbot/internal/domain/user"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*user.User, error) {
	var u user.User
	err := r.db.QueryRow(ctx, `SELECT id, username, created_at, banned FROM users WHERE id=$1`, id).
		Scan(&u.ID, &u.Username, &u.CreatedAt, &u.Banned)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) Upsert(ctx context.Context, u user.User) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO users(id, username, created_at, banned)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (id)
		DO UPDATE SET username=EXCLUDED.username, banned=EXCLUDED.banned
	`, u.ID, u.Username, u.CreatedAt, u.Banned)
	return err
}
