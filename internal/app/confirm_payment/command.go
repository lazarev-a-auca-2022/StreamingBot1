package confirm_payment

import "streamingbot/internal/domain/payment"

type Command struct {
	Event payment.Event
}
