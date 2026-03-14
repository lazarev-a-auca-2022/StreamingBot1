package review

import "time"

type Review struct {
	ID         string
	UserID     int64
	PurchaseID string
	Rating     int
	Text       string
	Published  bool
	CreatedAt  time.Time
}

func (r Review) IsValidRating() bool {
	return r.Rating >= 1 && r.Rating <= 5
}
