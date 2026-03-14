package use_access

type Command struct {
	Token string `json:"token"`
}

type Result struct {
	PurchaseID string `json:"purchase_id"`
	UserID     int64  `json:"user_id"`
	Valid      bool   `json:"valid"`
}
