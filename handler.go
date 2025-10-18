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

var defaultHTTPClient httpClient = &http.Client{Timeout: 10 * time.Second}

const errorWebhookEnvVar = "ERROR_WEBHOOK_URL"

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
	eventMap, err := normaliseEvent(event)
	if err != nil {
		notifyProcessingError(ctx, defaultHTTPClient, event, nil, err)
		return Response{}, err
	}

	webhookURL, err := extractWebhookURL(eventMap)
	if err != nil {
		notifyProcessingError(ctx, defaultHTTPClient, event, eventMap, err)
		return Response{}, err
	}

	payload, err := buildDiscordPayload(eventMap)
	if err != nil {
		notifyProcessingError(ctx, defaultHTTPClient, event, eventMap, err)
		return Response{}, err
	}

	status, body, err := sendDiscordMessage(ctx, defaultHTTPClient, webhookURL, payload)
	if err != nil {
		notifyProcessingError(ctx, defaultHTTPClient, event, eventMap, err)
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

func extractWebhookURL(event map[string]any) (string, error) {
	for _, key := range []string{"webhookURL", "webhook_url"} {
		if value, ok := event[key]; ok {
			if str := strings.TrimSpace(fmt.Sprint(value)); str != "" {
				return str, nil
			}
		}
	}

	return "", errors.New("event must contain a 'webhookURL' or 'webhook_url'")
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

func notifyProcessingError(
	ctx context.Context,
	client httpClient,
	rawEvent json.RawMessage,
	event map[string]any,
	procErr error,
) {
	if procErr == nil {
		return
	}

	webhookURL := strings.TrimSpace(os.Getenv(errorWebhookEnvVar))
	if webhookURL == "" {
		return
	}

	payload := buildErrorNotificationPayload(rawEvent, event, procErr)

	if client == nil {
		client = defaultHTTPClient
	}

	_, _, _ = sendDiscordMessage(ctx, client, webhookURL, payload)
}

func buildErrorNotificationPayload(rawEvent json.RawMessage, event map[string]any, procErr error) map[string]any {
	payload := map[string]any{
		"content": fmt.Sprintf("Failed to process request: %v", procErr),
	}

	if description := formatRequestForNotification(rawEvent, event); description != "" {
		payload["embeds"] = []map[string]string{
			{
				"title":       "Request",
				"description": description,
			},
		}
	}

	return payload
}

func formatRequestForNotification(rawEvent json.RawMessage, event map[string]any) string {
	if event != nil {
		if b, err := json.MarshalIndent(event, "", "  "); err == nil {
			return wrapAsJSONCodeBlock(b)
		}
	}

	trimmed := bytes.TrimSpace(rawEvent)
	if len(trimmed) == 0 {
		return ""
	}

	if json.Valid(trimmed) {
		var buf bytes.Buffer
		if err := json.Indent(&buf, trimmed, "", "  "); err == nil {
			return wrapAsJSONCodeBlock(buf.Bytes())
		}
		return wrapAsJSONCodeBlock(trimmed)
	}

	return fmt.Sprintf("```\n%s\n```", string(trimmed))
}

func wrapAsJSONCodeBlock(data []byte) string {
	return fmt.Sprintf("```json\n%s\n```", string(data))
}
