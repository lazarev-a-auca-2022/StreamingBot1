package purchase

type Status string

const (
	StatusPending      Status = "pending"
	StatusPaid         Status = "paid"
	StatusAccessIssued Status = "access_issued"
	StatusExpired      Status = "expired"
	StatusError        Status = "error"
)
