package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"lambda-to-discord/adapter"
	"lambda-to-discord/discord"
	"lambda-to-discord/domain"
)

var defaultHTTPClient discord.HTTPClient = &http.Client{Timeout: 10 * time.Second}

const (
	errorWebhookEnvVar      = "ERROR_WEBHOOK_URL"
	adapterTypeEnvVar       = "ADAPTER_TYPE"
	cloudWatchWebhookEnvVar = "WEBHOOK_URL"
)

type Response struct {
	StatusCode int    `json:"statusCode"`
	Body       string `json:"body"`
}

func HandleRequest(ctx context.Context, event json.RawMessage) (Response, error) {
	adapterType := strings.ToLower(strings.TrimSpace(os.Getenv(adapterTypeEnvVar)))

	payload, eventMap, err := buildNotificationPayload(adapterType, event)
	if err != nil {
		notifyProcessingError(ctx, defaultHTTPClient, event, eventMap, err)
		return Response{}, err
	}

	status, body, err := discord.Send(ctx, defaultHTTPClient, payload)
	if err != nil {
		notifyProcessingError(ctx, defaultHTTPClient, event, eventMap, err)
		return Response{}, err
	}

	return Response{StatusCode: status, Body: body}, nil
}

func buildNotificationPayload(adapterType string, event json.RawMessage) (domain.NotificationPayload, map[string]any, error) {
	switch adapterType {
	case "", "direct":
		payload, eventMap, err := adapter.NewDirectAdapter().Transform(event)
		return payload, eventMap, err
	case "cloudwatch":
		webhookURL := os.Getenv(cloudWatchWebhookEnvVar)
		payload, eventMap, err := adapter.NewCloudWatchSNSAdapter(webhookURL).Transform(event)
		return payload, eventMap, err
	default:
		return domain.NotificationPayload{}, nil, fmt.Errorf("unsupported adapter type: %s", adapterType)
	}
}

func notifyProcessingError(
	ctx context.Context,
	client discord.HTTPClient,
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

	payload := buildErrorNotificationPayload(webhookURL, rawEvent, event, procErr)

	if client == nil {
		client = defaultHTTPClient
	}

	_, _, _ = discord.Send(ctx, client, payload)
}

func buildErrorNotificationPayload(webhookURL string, rawEvent json.RawMessage, event map[string]any, procErr error) domain.NotificationPayload {
	payload := domain.NotificationPayload{
		WebhookURL:      webhookURL,
		Content:         fmt.Sprintf("Failed to process request: %v", procErr),
		AllowedMentions: domain.NoMentions(),
	}

	if description := formatRequestForNotification(rawEvent, event); description != "" {
		payload.Embeds = []domain.Embed{
			{
				Title:       "Request",
				Description: description,
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
