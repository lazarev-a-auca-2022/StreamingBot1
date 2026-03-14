package streaming

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is a Bunny Stream specific implementation.
type Client struct {
	httpClient      *http.Client
	apiBaseURL      string
	embedBaseURL    string
	libraryID       string
	apiKey          string
	tokenAuthSecret string
}

func NewBunnyClient(libraryID, apiKey, apiBaseURL, embedBaseURL, tokenAuthSecret string) Client {
	if strings.TrimSpace(apiBaseURL) == "" {
		apiBaseURL = "https://video.bunnycdn.com"
	}
	if strings.TrimSpace(embedBaseURL) == "" {
		embedBaseURL = "https://iframe.mediadelivery.net/embed"
	}
	return Client{
		httpClient:      &http.Client{Timeout: 8 * time.Second},
		apiBaseURL:      strings.TrimRight(apiBaseURL, "/"),
		embedBaseURL:    strings.TrimRight(embedBaseURL, "/"),
		libraryID:       libraryID,
		apiKey:          apiKey,
		tokenAuthSecret: tokenAuthSecret,
	}
}

func (c Client) IssueAccessLink(ctx context.Context, externalRef []byte, userID int64, ttl time.Duration, idempotencyKey string) (string, time.Time, error) {
	videoID := strings.TrimSpace(string(externalRef))
	if videoID == "" {
		return "", time.Time{}, fmt.Errorf("empty bunny video id")
	}
	if c.libraryID == "" {
		return "", time.Time{}, fmt.Errorf("missing bunny library id")
	}
	if c.apiKey == "" {
		return "", time.Time{}, fmt.Errorf("missing bunny api key")
	}

	// Validate video exists in Bunny Stream before issuing access.
	checkURL := fmt.Sprintf("%s/library/%s/videos/%s", c.apiBaseURL, url.PathEscape(c.libraryID), url.PathEscape(videoID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("AccessKey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("bunny api returned status %d", resp.StatusCode)
	}

	expiresAt := time.Now().Add(ttl)
	playURL := fmt.Sprintf("%s/%s/%s", c.embedBaseURL, url.PathEscape(c.libraryID), url.PathEscape(videoID))
	q := url.Values{}
	q.Set("uid", strconv.FormatInt(userID, 10))
	q.Set("pid", idempotencyKey)
	q.Set("expires", strconv.FormatInt(expiresAt.Unix(), 10))

	if c.tokenAuthSecret != "" {
		sigPayload := fmt.Sprintf("%s|%d|%d|%s", videoID, userID, expiresAt.Unix(), idempotencyKey)
		digest := sha256.Sum256([]byte(c.tokenAuthSecret + ":" + sigPayload))
		q.Set("sig", hex.EncodeToString(digest[:]))
	}

	return playURL + "?" + q.Encode(), expiresAt, nil
}
