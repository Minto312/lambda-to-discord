package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"lambda-to-discord/domain"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type WebhookError struct {
	Err error
}

func (e *WebhookError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("failed to send message to Discord: %v", e.Err)
}

func (e *WebhookError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func Send(ctx context.Context, client HTTPClient, payload domain.NotificationPayload) (int, string, error) {
	if err := payload.Validate(); err != nil {
		return 0, "", err
	}

	body, err := buildRequestBody(payload)
	if err != nil {
		return 0, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, payload.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", &WebhookError{Err: err}
	}

	respBody, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil {
		if closeErr != nil {
			readErr = errors.Join(readErr, fmt.Errorf("failed to close response body: %w", closeErr))
		}
		return 0, "", fmt.Errorf("failed to read response: %w", readErr)
	}

	if closeErr != nil {
		return 0, "", fmt.Errorf("failed to close response body: %w", closeErr)
	}

	return resp.StatusCode, string(respBody), nil
}

func buildRequestBody(payload domain.NotificationPayload) ([]byte, error) {
	body := map[string]any{}
	if strings.TrimSpace(payload.Content) != "" {
		body["content"] = payload.Content
	}
	if len(payload.Embeds) > 0 {
		body["embeds"] = payload.Embeds
	}
	if payload.AllowedMentions != nil {
		body["allowed_mentions"] = payload.AllowedMentions
	}
	if strings.TrimSpace(payload.Username) != "" {
		body["username"] = payload.Username
	}
	if strings.TrimSpace(payload.AvatarURL) != "" {
		body["avatar_url"] = payload.AvatarURL
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return encoded, nil
}
