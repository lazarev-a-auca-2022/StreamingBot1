package content

type Content struct {
	ID          string
	ExternalRef []byte
	Title       string
	Description string
	PriceStars  int
	Active      bool
}

func (c Content) CanBePurchased() bool {
	return c.Active && c.PriceStars > 0
}
