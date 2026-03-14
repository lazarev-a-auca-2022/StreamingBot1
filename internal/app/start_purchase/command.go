package start_purchase

type Command struct {
	UserID    int64  `json:"user_id"`
	ContentID string `json:"content_id"`
}

type Result struct {
	PurchaseID     string `json:"purchase_id"`
	InvoicePayload string `json:"invoice_payload"`
	AmountStars    int    `json:"amount_stars"`
}
