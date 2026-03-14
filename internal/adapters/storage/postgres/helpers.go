package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrNoRows = pgx.ErrNoRows

func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrNoRows)
}
