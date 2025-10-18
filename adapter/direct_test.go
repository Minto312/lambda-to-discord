package adapter

import (
	"encoding/json"
	"testing"
)

func TestDirectAdapterTransformSuccess(t *testing.T) {
	raw := json.RawMessage(`{
                "webhookURL": "https://discord.example/hook",
                "content": "hello",
                "username": "bot",
                "avatar_url": "https://example.com/avatar.png",
                "embeds": [{"title": "Example"}],
                "allowed_mentions": {"parse": []}
        }`)

	payload, eventMap, err := NewDirectAdapter().Transform(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.WebhookURL != "https://discord.example/hook" {
		t.Fatalf("unexpected webhook: %s", payload.WebhookURL)
	}
	if payload.Content != "hello" {
		t.Fatalf("unexpected content: %s", payload.Content)
	}
	if payload.Username != "bot" || payload.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("unexpected identity: %#v", payload)
	}
	if len(payload.Embeds) != 1 || payload.Embeds[0].Title != "Example" {
		t.Fatalf("unexpected embeds: %#v", payload.Embeds)
	}
	if payload.AllowedMentions != nil {
		t.Fatalf("expected allowed mentions to be nil when empty, got %#v", payload.AllowedMentions)
	}
	if eventMap["content"].(string) != "hello" {
		t.Fatalf("event map not preserved: %#v", eventMap)
	}
}

func TestDirectAdapterTransformAllowsMessageFallback(t *testing.T) {
	raw := json.RawMessage(`{"webhook_url": "https://discord.example/hook", "message": "fallback"}`)
	payload, _, err := NewDirectAdapter().Transform(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.Content != "fallback" {
		t.Fatalf("expected fallback content, got %s", payload.Content)
	}
}

func TestDirectAdapterTransformError(t *testing.T) {
	if _, _, err := NewDirectAdapter().Transform(json.RawMessage(`{"content":"missing webhook"}`)); err == nil {
		t.Fatal("expected error when webhook missing")
	}
	if _, _, err := NewDirectAdapter().Transform(json.RawMessage(`123`)); err == nil {
		t.Fatal("expected error for invalid payload")
	}
	if _, _, err := NewDirectAdapter().Transform(json.RawMessage(`{"webhookURL":"https://hook"}`)); err == nil {
		t.Fatal("expected error when content missing")
	}
}

func TestDirectAdapterTransformAllowedMentions(t *testing.T) {
	raw := json.RawMessage(`{
                "webhookURL": "https://discord.example/hook",
                "content": "hi",
                "allowed_mentions": {
                        "parse": ["users"],
                        "users": ["123"],
                        "replied_user": true
                }
        }`)
	payload, _, err := NewDirectAdapter().Transform(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.AllowedMentions == nil {
		t.Fatalf("expected allowed mentions to be set")
	}
	if !payload.AllowedMentions.RepliedUser {
		t.Fatalf("expected replied_user to be true")
	}
	if len(payload.AllowedMentions.Users) != 1 || payload.AllowedMentions.Users[0] != "123" {
		t.Fatalf("unexpected allowed mentions users: %#v", payload.AllowedMentions.Users)
	}
}
