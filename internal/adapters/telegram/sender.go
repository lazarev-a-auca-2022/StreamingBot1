package telegram

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Sender struct {
	bot *tgbotapi.BotAPI
}

func NewSender(bot *tgbotapi.BotAPI) Sender {
	return Sender{bot: bot}
}

func (s Sender) SendAccessLink(ctx context.Context, userID int64, link string) error {
	_ = ctx
	if s.bot == nil {
		return nil
	}
	msg := tgbotapi.NewMessage(userID, fmt.Sprintf("Your access link is ready:\n%s", link))
	_, err := s.bot.Send(msg)
	return err
}

func (s Sender) SendReviewRequest(ctx context.Context, userID int64, purchaseID string) error {
	_ = ctx
	if s.bot == nil {
		return nil
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1", fmt.Sprintf("review:%s:1", purchaseID)),
			tgbotapi.NewInlineKeyboardButtonData("2", fmt.Sprintf("review:%s:2", purchaseID)),
			tgbotapi.NewInlineKeyboardButtonData("3", fmt.Sprintf("review:%s:3", purchaseID)),
			tgbotapi.NewInlineKeyboardButtonData("4", fmt.Sprintf("review:%s:4", purchaseID)),
			tgbotapi.NewInlineKeyboardButtonData("5", fmt.Sprintf("review:%s:5", purchaseID)),
		),
	)
	msg := tgbotapi.NewMessage(userID, "Please rate your purchase")
	msg.ReplyMarkup = keyboard
	_, err := s.bot.Send(msg)
	return err
}
