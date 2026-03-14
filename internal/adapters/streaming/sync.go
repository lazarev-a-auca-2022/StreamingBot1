package streaming

import (
	"context"
	"fmt"
	"streamingbot/internal/domain/content"
	"strings"
)

type contentSyncRepository interface {
	GetByID(ctx context.Context, id string) (*content.Content, error)
	Upsert(ctx context.Context, c content.Content) error
}

func SyncLibraryContent(ctx context.Context, client Client, repo contentSyncRepository, defaultPrice int) (int, error) {
	if defaultPrice <= 0 {
		defaultPrice = 25
	}

	page := 1
	synced := 0
	for {
		videos, err := client.ListLibraryVideos(ctx, page, 100)
		if err != nil {
			return synced, err
		}
		if len(videos) == 0 {
			break
		}

		for _, v := range videos {
			id := strings.TrimSpace(v.GUID)
			if id == "" {
				continue
			}

			existing, err := repo.GetByID(ctx, id)
			if err != nil {
				return synced, err
			}

			item := content.Content{
				ID:          id,
				ExternalRef: []byte(id),
				Title:       fallback(v.Title, fmt.Sprintf("Video %s", id)),
				Description: strings.TrimSpace(v.Description),
				PriceStars:  defaultPrice,
				Active:      true,
			}
			if existing != nil {
				item.PriceStars = existing.PriceStars
				item.Active = existing.Active
				if existing.Description != "" {
					item.Description = existing.Description
				}
			}

			if err := repo.Upsert(ctx, item); err != nil {
				return synced, err
			}
			synced++
		}

		if len(videos) < 100 {
			break
		}
		page++
	}

	return synced, nil
}

func fallback(v, fb string) string {
	if strings.TrimSpace(v) == "" {
		return fb
	}
	return v
}
