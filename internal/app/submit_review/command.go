package submit_review

type Command struct {
	UserID     int64  `json:"user_id"`
	PurchaseID string `json:"purchase_id"`
	Rating     int    `json:"rating"`
	Text       string `json:"text"`
}
