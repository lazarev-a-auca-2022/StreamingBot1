package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

func NewClient(baseURL, apiKey string) Client {
	return Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    baseURL,
		apiKey:     apiKey,
	}
}

func (c Client) IssueAccessLink(ctx context.Context, externalRef []byte, userID int64, ttl time.Duration, idempotencyKey string) (string, time.Time, error) {
	if c.baseURL == "" {
		expiresAt := time.Now().Add(ttl)
		return "https://example.invalid/watch", expiresAt, nil
	}

	reqBody := map[string]any{
		"external_ref":    string(externalRef),
		"user_id":         userID,
		"ttl_minutes":     int(ttl.Minutes()),
		"idempotency_key": idempotencyKey,
	}
	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/access/issue-link", bytes.NewReader(b))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	var out struct {
		Link      string    `json:"link"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return "", time.Time{}, errors.New("streaming provider returned non-success")
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", time.Time{}, err
	}
	if out.Link == "" {
		return "", time.Time{}, errors.New("streaming provider returned empty link")
	}
	if out.ExpiresAt.IsZero() {
		out.ExpiresAt = time.Now().Add(ttl)
	}
	return out.Link, out.ExpiresAt, nil
}
