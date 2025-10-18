package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const webhookEnvVar = "DISCORD_WEBHOOK_URL"

var defaultHTTPClient httpClient = &http.Client{Timeout: 10 * time.Second}

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Response mirrors the structure expected by API Gateway Lambda integrations.
type Response struct {
	StatusCode int    `json:"statusCode"`
	Body       string `json:"body"`
}

// WebhookError captures failures returned by the Discord webhook endpoint.
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

// HandleRequest is the Lambda entrypoint.
func HandleRequest(ctx context.Context, event json.RawMessage) (Response, error) {
	webhookURL := strings.TrimSpace(os.Getenv(webhookEnvVar))
	if webhookURL == "" {
		return Response{}, fmt.Errorf("environment variable %q is required", webhookEnvVar)
	}

	eventMap, err := normaliseEvent(event)
	if err != nil {
		return Response{}, err
	}

	payload, err := buildDiscordPayload(eventMap)
	if err != nil {
		return Response{}, err
	}

	status, body, err := sendDiscordMessage(ctx, defaultHTTPClient, webhookURL, payload)
	if err != nil {
		return Response{}, err
	}

	return Response{StatusCode: status, Body: body}, nil
}

func normaliseEvent(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return map[string]any{}, nil
	}

	var eventMap map[string]any
	if err := json.Unmarshal(trimmed, &eventMap); err == nil {
		return eventMap, nil
	}

	var asString string
	if err := json.Unmarshal(trimmed, &asString); err == nil {
		inner := strings.TrimSpace(asString)
		if inner == "" {
			return map[string]any{}, nil
		}
		if err := json.Unmarshal([]byte(inner), &eventMap); err != nil {
			return nil, fmt.Errorf("event string must contain valid JSON object: %w", err)
		}
		return eventMap, nil
	}

	return nil, errors.New("event must be an object or JSON string")
}

func buildDiscordPayload(event map[string]any) (map[string]any, error) {
	payload := make(map[string]any)

	if content, ok := event["content"]; ok {
		if str := strings.TrimSpace(fmt.Sprint(content)); str != "" {
			payload["content"] = str
		}
	}
	if _, ok := payload["content"]; !ok {
		if message, ok := event["message"]; ok {
			if str := strings.TrimSpace(fmt.Sprint(message)); str != "" {
				payload["content"] = str
			}
		}
	}

	if _, ok := payload["content"]; !ok {
		return nil, errors.New("event must contain a 'content' or 'message' field")
	}

	if username, ok := event["username"]; ok {
		if str := strings.TrimSpace(fmt.Sprint(username)); str != "" {
			payload["username"] = str
		}
	}
	if avatar, ok := event["avatar_url"]; ok {
		if str := strings.TrimSpace(fmt.Sprint(avatar)); str != "" {
			payload["avatar_url"] = str
		}
	}
	if embeds, ok := event["embeds"]; ok {
		switch v := embeds.(type) {
		case json.RawMessage:
			payload["embeds"] = json.RawMessage(v)
		default:
			payload["embeds"] = v
		}
	}

	return payload, nil
}

func sendDiscordMessage(
	ctx context.Context,
	client httpClient,
	webhookURL string,
	payload map[string]any,
) (int, string, error) {
	if strings.TrimSpace(webhookURL) == "" {
		return 0, "", errors.New("discord webhook URL must be provided")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", &WebhookError{Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("failed to read response: %w", err)
	}

	return resp.StatusCode, string(respBody), nil
}
