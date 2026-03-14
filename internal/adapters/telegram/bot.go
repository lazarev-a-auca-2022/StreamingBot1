package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"streamingbot/internal/app/confirm_payment"
	"streamingbot/internal/app/start_purchase"
	"streamingbot/internal/app/submit_review"
	"streamingbot/internal/domain/content"
	"streamingbot/internal/domain/payment"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api          *tgbotapi.BotAPI
	catalog      content.Repository
	start        start_purchase.Handler
	confirm      confirm_payment.Handler
	submitReview submit_review.Handler
	pollTimeout  int
	adminSecret  string
	adminMu      sync.RWMutex
	adminUsers   map[int64]bool
}

type contentAdminRepository interface {
	Upsert(ctx context.Context, c content.Content) error
	DeleteByID(ctx context.Context, id string) error
}

func NewBot(token string, pollTimeout int, adminSecret string, catalog content.Repository, start start_purchase.Handler, confirm confirm_payment.Handler, submit submit_review.Handler) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	if pollTimeout <= 0 {
		pollTimeout = 30
	}
	return &Bot{
		api:          api,
		catalog:      catalog,
		start:        start,
		confirm:      confirm,
		submitReview: submit,
		pollTimeout:  pollTimeout,
		adminSecret:  adminSecret,
		adminUsers:   map[int64]bool{},
	}, nil
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
	if up.PreCheckoutQuery != nil {
		b.handlePreCheckout(ctx, up.PreCheckoutQuery)
		return
	}
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

	if msg.SuccessfulPayment != nil {
		sp := msg.SuccessfulPayment
		err := b.confirm.Handle(ctx, confirm_payment.Command{Event: payment.Event{
			ChargeID:       sp.TelegramPaymentChargeID,
			AmountStars:    sp.TotalAmount,
			InvoicePayload: sp.InvoicePayload,
			RawPayload:     []byte(fmt.Sprintf(`{"currency":"%s","telegram_payment_charge_id":"%s"}`, sp.Currency, sp.TelegramPaymentChargeID)),
			OccurredAt:     time.Now(),
		}})
		if err != nil {
			b.reply(chatID, "Payment received, but confirmation failed. Please contact support.")
			return
		}
		b.reply(chatID, "Payment received. Preparing your access link...")
		return
	}

	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			b.reply(chatID, "Welcome. Use /catalog to see available videos.")
		case "help":
			b.reply(chatID, "Commands:\n/catalog\n/buy <content_id>\n/review <purchase_id> <rating> [text]")
		case "adminmode":
			secret := strings.TrimSpace(msg.CommandArguments())
			if secret == "" || secret != b.adminSecret {
				b.reply(chatID, "admin denied")
				return
			}
			enabled := b.toggleAdmin(userID)
			if enabled {
				b.reply(chatID, "admin mode enabled")
			} else {
				b.reply(chatID, "admin mode disabled")
			}
		case "createcontent":
			if !b.isAdmin(userID) {
				b.reply(chatID, "admin required")
				return
			}
			// Usage: /createcontent <id> <bunny_video_id> <price> <title>|<description>
			args := strings.TrimSpace(msg.CommandArguments())
			parts := strings.Fields(args)
			if len(parts) < 4 {
				b.reply(chatID, "Usage: /createcontent <id> <bunny_video_id> <price> <title>|<description>")
				return
			}
			price, err := strconv.Atoi(parts[2])
			if err != nil || price <= 0 {
				b.reply(chatID, "price must be positive integer")
				return
			}
			meta := strings.Join(parts[3:], " ")
			title, desc := splitTitleDesc(meta)
			if title == "" {
				title = parts[0]
			}
			if err := b.upsertContent(ctx, content.Content{
				ID:          parts[0],
				ExternalRef: []byte(parts[1]),
				Title:       title,
				Description: desc,
				PriceStars:  price,
				Active:      true,
			}); err != nil {
				b.reply(chatID, "create content failed")
				return
			}
			b.reply(chatID, "content created")
		case "deletecontent":
			if !b.isAdmin(userID) {
				b.reply(chatID, "admin required")
				return
			}
			contentID := strings.TrimSpace(msg.CommandArguments())
			if contentID == "" {
				b.reply(chatID, "Usage: /deletecontent <id>")
				return
			}
			if err := b.deleteContent(ctx, contentID); err != nil {
				b.reply(chatID, "delete content failed")
				return
			}
			b.reply(chatID, "content deleted")
		case "setcontent":
			if !b.isAdmin(userID) {
				b.reply(chatID, "admin required")
				return
			}
			// Usage: /setcontent <id> <price> <title>|<description>
			args := strings.TrimSpace(msg.CommandArguments())
			parts := strings.Fields(args)
			if len(parts) < 3 {
				b.reply(chatID, "Usage: /setcontent <id> <price> <title>|<description>")
				return
			}
			price, err := strconv.Atoi(parts[1])
			if err != nil || price <= 0 {
				b.reply(chatID, "price must be positive integer")
				return
			}
			existing, err := b.catalog.GetByID(ctx, parts[0])
			if err != nil || existing == nil {
				b.reply(chatID, "content not found")
				return
			}
			meta := strings.Join(parts[2:], " ")
			title, desc := splitTitleDesc(meta)
			if title != "" {
				existing.Title = title
			}
			if desc != "" {
				existing.Description = desc
			}
			existing.PriceStars = price
			if err := b.upsertContent(ctx, *existing); err != nil {
				b.reply(chatID, "update content failed")
				return
			}
			b.reply(chatID, "content updated")
		case "forcebuy":
			if !b.isAdmin(userID) {
				b.reply(chatID, "admin required")
				return
			}
			contentID := strings.TrimSpace(msg.CommandArguments())
			if contentID == "" {
				b.reply(chatID, "Usage: /forcebuy <content_id>")
				return
			}
			if err := b.forceBuy(ctx, chatID, userID, contentID); err != nil {
				b.reply(chatID, "forcebuy failed: "+err.Error())
				return
			}
			b.reply(chatID, "forcebuy success: purchase marked paid; access link will be sent shortly")
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
			lines := []string{"Catalog:"}
			for _, c := range items {
				line := fmt.Sprintf("- %s (%s): %d⭐", c.Title, c.ID, c.PriceStars)
				if c.Description != "" {
					line += "\n  " + c.Description
				}
				lines = append(lines, line)
			}
			m := tgbotapi.NewMessage(chatID, strings.Join(lines, "\n"))
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

	item, _ := b.catalog.GetByID(ctx, contentID)
	title := "Streaming Content"
	desc := "Access to purchased content"
	if item != nil {
		if strings.TrimSpace(item.Title) != "" {
			title = item.Title
		}
		if strings.TrimSpace(item.Description) != "" {
			desc = item.Description
		}
	}

	prices := []tgbotapi.LabeledPrice{{Label: title, Amount: res.AmountStars}}
	invoice := tgbotapi.NewInvoice(
		chatID,
		title,
		desc,
		res.InvoicePayload,
		"", // provider token not used for XTR
		"purchase_"+res.PurchaseID,
		"XTR",
		prices,
	)

	if _, err := b.api.Send(invoice); err != nil {
		b.reply(chatID, fmt.Sprintf("Purchase created, but invoice send failed. Purchase ID: %s", res.PurchaseID))
		return
	}
	b.reply(chatID, fmt.Sprintf("Purchase created.\nPurchase ID: %s\nPlease complete Telegram Stars payment.", res.PurchaseID))
}

func (b *Bot) ack(callbackID, text string) {
	_, _ = b.api.Request(tgbotapi.NewCallback(callbackID, text))
}

func (b *Bot) reply(chatID int64, text string) {
	m := tgbotapi.NewMessage(chatID, text)
	_, _ = b.api.Send(m)
}

func (b *Bot) isAdmin(userID int64) bool {
	b.adminMu.RLock()
	defer b.adminMu.RUnlock()
	return b.adminUsers[userID]
}

func (b *Bot) toggleAdmin(userID int64) bool {
	b.adminMu.Lock()
	defer b.adminMu.Unlock()
	b.adminUsers[userID] = !b.adminUsers[userID]
	return b.adminUsers[userID]
}

func (b *Bot) upsertContent(ctx context.Context, c content.Content) error {
	repo, ok := b.catalog.(contentAdminRepository)
	if !ok {
		return fmt.Errorf("catalog repository does not support admin writes")
	}
	return repo.Upsert(ctx, c)
}

func (b *Bot) deleteContent(ctx context.Context, id string) error {
	repo, ok := b.catalog.(contentAdminRepository)
	if !ok {
		return fmt.Errorf("catalog repository does not support admin writes")
	}
	return repo.DeleteByID(ctx, id)
}

func splitTitleDesc(raw string) (string, string) {
	parts := strings.SplitN(raw, "|", 2)
	title := strings.TrimSpace(parts[0])
	desc := ""
	if len(parts) > 1 {
		desc = strings.TrimSpace(parts[1])
	}
	return title, desc
}

func (b *Bot) handlePreCheckout(ctx context.Context, q *tgbotapi.PreCheckoutQuery) {
	if q == nil {
		return
	}
	resp := tgbotapi.PreCheckoutConfig{
		PreCheckoutQueryID: q.ID,
		OK:                 false,
	}

	if q.Currency != "XTR" {
		resp.ErrorMessage = "Unsupported currency"
		_, _ = b.api.Request(resp)
		return
	}

	p, err := b.start.Purchases.GetByPayload(ctx, q.InvoicePayload)
	if err != nil || p == nil {
		resp.ErrorMessage = "Invoice not found"
		_, _ = b.api.Request(resp)
		return
	}
	if p.UserID != q.From.ID {
		resp.ErrorMessage = "Invoice owner mismatch"
		_, _ = b.api.Request(resp)
		return
	}
	if p.StarsAmount != q.TotalAmount {
		resp.ErrorMessage = "Price mismatch"
		_, _ = b.api.Request(resp)
		return
	}

	resp.OK = true
	resp.ErrorMessage = ""
	_, _ = b.api.Request(resp)
}

func (b *Bot) forceBuy(ctx context.Context, chatID int64, userID int64, contentID string) error {
	res, err := b.start.Handle(ctx, start_purchase.Command{UserID: userID, ContentID: contentID})
	if err != nil {
		return err
	}

	chargeID := fmt.Sprintf("forcebuy-%d-%d", userID, time.Now().UnixNano())
	err = b.confirm.Handle(ctx, confirm_payment.Command{Event: payment.Event{
		ChargeID:       chargeID,
		AmountStars:    res.AmountStars,
		InvoicePayload: res.InvoicePayload,
		RawPayload:     []byte(`{"source":"forcebuy"}`),
		OccurredAt:     time.Now(),
	}})
	if err != nil {
		return err
	}

	b.reply(chatID, fmt.Sprintf("Force purchase created and paid.\nPurchase ID: %s", res.PurchaseID))
	return nil
}
