package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"streamingbot/internal/app/start_purchase"
	"streamingbot/internal/app/submit_review"
	"streamingbot/internal/domain/content"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api          *tgbotapi.BotAPI
	catalog      content.Repository
	start        start_purchase.Handler
	submitReview submit_review.Handler
	pollTimeout  int
}

func NewBot(token string, pollTimeout int, catalog content.Repository, start start_purchase.Handler, submit submit_review.Handler) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	if pollTimeout <= 0 {
		pollTimeout = 30
	}
	return &Bot{api: api, catalog: catalog, start: start, submitReview: submit, pollTimeout: pollTimeout}, nil
}

func (b *Bot) API() *tgbotapi.BotAPI {
	return b.api
}

func (b *Bot) SetMenu(ctx context.Context) error {
	_ = ctx
	cmds := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "start", Description: "Start bot"},
		tgbotapi.BotCommand{Command: "catalog", Description: "Show content catalog"},
		tgbotapi.BotCommand{Command: "buy", Description: "Buy content by ID: /buy <content_id>"},
		tgbotapi.BotCommand{Command: "review", Description: "Submit review: /review <purchase_id> <rating> [text]"},
		tgbotapi.BotCommand{Command: "help", Description: "Show help"},
	)
	_, err := b.api.Request(cmds)
	return err
}

func (b *Bot) Start(ctx context.Context) error {
	if err := b.SetMenu(ctx); err != nil {
		return err
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = b.pollTimeout
	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return nil
		case up := <-updates:
			b.handleUpdate(ctx, up)
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, up tgbotapi.Update) {
	if up.Message != nil {
		b.handleMessage(ctx, up.Message)
		return
	}
	if up.CallbackQuery != nil {
		b.handleCallback(ctx, up.CallbackQuery)
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	if msg == nil {
		return
	}
	chatID := msg.Chat.ID
	userID := msg.From.ID

	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			b.reply(chatID, "Welcome. Use /catalog to see available videos.")
		case "help":
			b.reply(chatID, "Commands:\n/catalog\n/buy <content_id>\n/review <purchase_id> <rating> [text]")
		case "catalog":
			items, err := b.catalog.ListActive(ctx)
			if err != nil {
				b.reply(chatID, "Failed to load catalog")
				return
			}
			if len(items) == 0 {
				b.reply(chatID, "Catalog is empty")
				return
			}
			buttons := make([][]tgbotapi.InlineKeyboardButton, 0, len(items))
			for _, c := range items {
				label := fmt.Sprintf("Buy %s (%d⭐)", c.Title, c.PriceStars)
				buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(label, "buy:"+c.ID),
				))
			}
			m := tgbotapi.NewMessage(chatID, "Catalog")
			m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
			_, _ = b.api.Send(m)
		case "buy":
			arg := strings.TrimSpace(msg.CommandArguments())
			if arg == "" {
				b.reply(chatID, "Usage: /buy <content_id>")
				return
			}
			b.startPurchase(ctx, chatID, userID, arg)
		case "review":
			args := strings.Fields(msg.CommandArguments())
			if len(args) < 2 {
				b.reply(chatID, "Usage: /review <purchase_id> <rating> [text]")
				return
			}
			rating, err := strconv.Atoi(args[1])
			if err != nil {
				b.reply(chatID, "Rating must be 1-5")
				return
			}
			text := ""
			if len(args) > 2 {
				text = strings.Join(args[2:], " ")
			}
			err = b.submitReview.Handle(ctx, submit_review.Command{
				UserID:     userID,
				PurchaseID: args[0],
				Rating:     rating,
				Text:       text,
			})
			if err != nil {
				b.reply(chatID, "Could not save review")
				return
			}
			b.reply(chatID, "Review saved. Thank you.")
		default:
			b.reply(chatID, "Unknown command. Use /help")
		}
		return
	}
}

func (b *Bot) handleCallback(ctx context.Context, cq *tgbotapi.CallbackQuery) {
	if cq == nil {
		return
	}
	chatID := cq.Message.Chat.ID
	userID := cq.From.ID
	data := cq.Data

	if strings.HasPrefix(data, "buy:") {
		contentID := strings.TrimPrefix(data, "buy:")
		b.startPurchase(ctx, chatID, userID, contentID)
		b.ack(cq.ID, "Purchase started")
		return
	}
	if strings.HasPrefix(data, "review:") {
		parts := strings.Split(data, ":")
		if len(parts) == 3 {
			rating, err := strconv.Atoi(parts[2])
			if err == nil {
				_ = b.submitReview.Handle(ctx, submit_review.Command{
					UserID:     userID,
					PurchaseID: parts[1],
					Rating:     rating,
					Text:       "rating from inline keyboard",
				})
				b.reply(chatID, "Review saved. Thank you.")
			}
		}
		b.ack(cq.ID, "Review submitted")
	}
}

func (b *Bot) startPurchase(ctx context.Context, chatID int64, userID int64, contentID string) {
	res, err := b.start.Handle(ctx, start_purchase.Command{UserID: userID, ContentID: contentID})
	if err != nil {
		b.reply(chatID, "Could not start purchase. Check content ID.")
		return
	}
	b.reply(chatID, fmt.Sprintf("Purchase created.\nPurchase ID: %s\nPrice: %d⭐\nWaiting for payment confirmation.", res.PurchaseID, res.AmountStars))
}

func (b *Bot) ack(callbackID, text string) {
	_, _ = b.api.Request(tgbotapi.NewCallback(callbackID, text))
}

func (b *Bot) reply(chatID int64, text string) {
	m := tgbotapi.NewMessage(chatID, text)
	_, _ = b.api.Send(m)
}
