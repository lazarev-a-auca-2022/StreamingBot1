package payment

import "time"

type Event struct {
	ChargeID       string
	AmountStars    int
	InvoicePayload string
	RawPayload     []byte
	OccurredAt     time.Time
}
