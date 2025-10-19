package adapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"lambda-to-discord/domain"
)

type DirectAdapter struct{}

func NewDirectAdapter() DirectAdapter {
	return DirectAdapter{}
}

func (DirectAdapter) Transform(event json.RawMessage) (domain.NotificationPayload, map[string]any, error) {
	eventMap, err := normaliseEvent(event)
	if err != nil {
		return domain.NotificationPayload{}, nil, err
	}

	webhookURL, err := extractWebhookURL(eventMap)
	if err != nil {
		fallback := strings.TrimSpace(os.Getenv("WEBHOOK_URL"))
		if fallback == "" {
			return domain.NotificationPayload{}, eventMap, err
		}
		webhookURL = fallback
	}

	content := extractFirstNonEmpty(eventMap, "content", "message")
	if content == "" {
		return domain.NotificationPayload{}, eventMap, errors.New("event must contain a 'content' or 'message' field")
	}

	payload := domain.NotificationPayload{
		WebhookURL: webhookURL,
		Content:    content,
	}

	if username := extractString(eventMap["username"]); username != "" {
		payload.Username = username
	}
	if avatar := extractString(eventMap["avatar_url"]); avatar != "" {
		payload.AvatarURL = avatar
	}

	if embeds, ok := eventMap["embeds"]; ok {
		parsed, err := parseEmbeds(embeds)
		if err != nil {
			return domain.NotificationPayload{}, eventMap, err
		}
		payload.Embeds = parsed
	}

	if allowed, ok := eventMap["allowed_mentions"]; ok {
		mentions, err := parseAllowedMentions(allowed)
		if err != nil {
			return domain.NotificationPayload{}, eventMap, err
		}
		payload.AllowedMentions = mentions
	}

	return payload, eventMap, nil
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

func extractWebhookURL(event map[string]any) (string, error) {
	for _, key := range []string{"webhookURL", "webhook_url"} {
		if value, ok := event[key]; ok {
			if str := extractString(value); str != "" {
				return str, nil
			}
		}
	}

	return "", errors.New("event must contain a 'webhookURL' or 'webhook_url'")
}

func extractFirstNonEmpty(event map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := event[key]; ok {
			if str := extractString(value); str != "" {
				return str
			}
		}
	}
	return ""
}

func extractString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func parseEmbeds(raw any) ([]domain.Embed, error) {
	if raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case json.RawMessage:
		return parseEmbeds([]byte(v))
	case []byte:
		var embeds []domain.Embed
		if err := json.Unmarshal(v, &embeds); err != nil {
			return nil, fmt.Errorf("embeds must be an array: %w", err)
		}
		return embeds, nil
	default:
		marshalled, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal embeds: %w", err)
		}
		var embeds []domain.Embed
		if err := json.Unmarshal(marshalled, &embeds); err != nil {
			return nil, fmt.Errorf("embeds must be an array: %w", err)
		}
		return embeds, nil
	}
}

func parseAllowedMentions(raw any) (*domain.AllowedMentions, error) {
	if raw == nil {
		return nil, nil
	}
	marshalled, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal allowed_mentions: %w", err)
	}
	var mentions domain.AllowedMentions
	if err := json.Unmarshal(marshalled, &mentions); err != nil {
		return nil, fmt.Errorf("allowed_mentions must follow Discord schema: %w", err)
	}
	if mentions.IsZero() {
		return nil, nil
	}
	return &mentions, nil
}
