package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type stubHTTPClient struct {
	req  *http.Request
	resp *http.Response
	err  error
}

func (s *stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	s.req = req
	if s.err != nil {
		return nil, s.err
	}
	if s.resp == nil {
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	}
	return s.resp, nil
}

func TestNormaliseEventObject(t *testing.T) {
	raw := json.RawMessage(`{"content":"hello"}`)
	event, err := normaliseEvent(raw)
	if err != nil {
		t.Fatalf("normaliseEvent returned error: %v", err)
	}
	if event["content"].(string) != "hello" {
		t.Fatalf("unexpected content: %#v", event["content"])
	}
}

func TestNormaliseEventJSONString(t *testing.T) {
	raw := json.RawMessage(`"{\"message\": \"hi\"}"`)
	event, err := normaliseEvent(raw)
	if err != nil {
		t.Fatalf("normaliseEvent returned error: %v", err)
	}
	if event["message"].(string) != "hi" {
		t.Fatalf("unexpected message: %#v", event["message"])
	}
}

func TestNormaliseEventInvalid(t *testing.T) {
	_, err := normaliseEvent(json.RawMessage(`123`))
	if err == nil {
		t.Fatal("expected error for invalid payload")
	}
}

func TestBuildDiscordPayload(t *testing.T) {
	payload, err := buildDiscordPayload(map[string]any{
		"message":    "hello",
		"username":   "bot",
		"avatar_url": "http://example.com/avatar.png",
		"embeds": []map[string]string{
			{"title": "Example"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload["content"].(string) != "hello" {
		t.Fatalf("expected content fallback, got %#v", payload["content"])
	}
	if payload["username"].(string) != "bot" {
		t.Fatalf("unexpected username: %#v", payload["username"])
	}
	if payload["avatar_url"].(string) != "http://example.com/avatar.png" {
		t.Fatalf("unexpected avatar: %#v", payload["avatar_url"])
	}
	embeds, ok := payload["embeds"].([]map[string]string)
	if !ok || embeds[0]["title"] != "Example" {
		t.Fatalf("unexpected embeds: %#v", payload["embeds"])
	}
}

func TestBuildDiscordPayloadMissingContent(t *testing.T) {
	_, err := buildDiscordPayload(map[string]any{})
	if err == nil {
		t.Fatal("expected error when content missing")
	}
}

func TestSendDiscordMessageSuccess(t *testing.T) {
	stub := &stubHTTPClient{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
		},
	}
	status, body, err := sendDiscordMessage(context.Background(), stub, "http://example.com", map[string]any{"content": "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("unexpected status: %d", status)
	}
	if body != "ok" {
		t.Fatalf("unexpected body: %q", body)
	}
	if stub.req == nil {
		t.Fatal("expected request to be recorded")
	}
	if stub.req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected content type: %q", stub.req.Header.Get("Content-Type"))
	}
	payloadBytes, err := io.ReadAll(stub.req.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}
	if cerr := stub.req.Body.Close(); cerr != nil {
		t.Fatalf("failed to close request body: %v", cerr)
	}
	if !strings.Contains(string(payloadBytes), "hi") {
		t.Fatalf("payload not sent: %q", string(payloadBytes))
	}
}

func TestSendDiscordMessageNetworkError(t *testing.T) {
	stub := &stubHTTPClient{err: errors.New("boom")}
	_, _, err := sendDiscordMessage(context.Background(), stub, "http://example.com", map[string]any{"content": "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
	var webhookErr *WebhookError
	if !errors.As(err, &webhookErr) {
		t.Fatalf("expected WebhookError, got %T", err)
	}
}

func TestHandleRequestSuccess(t *testing.T) {
	oldClient := defaultHTTPClient
	stub := &stubHTTPClient{
		resp: &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
		},
	}
	defaultHTTPClient = stub
	t.Cleanup(func() { defaultHTTPClient = oldClient })

	event := map[string]any{
		"content":    "hello",
		"webhookURL": "http://example.com",
	}
	raw, _ := json.Marshal(event)

	resp, err := HandleRequest(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestHandleRequestMissingWebhookURL(t *testing.T) {
	_, err := HandleRequest(context.Background(), json.RawMessage(`{"content":"hi"}`))
	if err == nil {
		t.Fatal("expected error when webhookURL missing")
	}
}

func TestHandleRequestInvalidEvent(t *testing.T) {
	_, err := HandleRequest(context.Background(), json.RawMessage(`123`))
	if err == nil {
		t.Fatal("expected error for invalid event")
	}
}

func TestHandleRequestNotifiesOnError(t *testing.T) {
	oldClient := defaultHTTPClient
	stub := &stubHTTPClient{
		resp: &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
		},
	}
	defaultHTTPClient = stub
	t.Cleanup(func() { defaultHTTPClient = oldClient })

	t.Setenv(errorWebhookEnvVar, "http://example.com/error")

	_, err := HandleRequest(context.Background(), json.RawMessage(`123`))
	if err == nil {
		t.Fatal("expected error for invalid event")
	}

	if stub.req == nil {
		t.Fatal("expected error notification request to be sent")
	}
	if stub.req.URL.String() != "http://example.com/error" {
		t.Fatalf("unexpected error webhook url: %s", stub.req.URL.String())
	}
	body, readErr := io.ReadAll(stub.req.Body)
	if readErr != nil {
		t.Fatalf("failed to read request body: %v", readErr)
	}
	if cerr := stub.req.Body.Close(); cerr != nil {
		t.Fatalf("failed to close request body: %v", cerr)
	}
	payload := string(body)
	if !strings.Contains(payload, "Failed to process request") {
		t.Fatalf("expected error message in payload: %s", payload)
	}
	if !strings.Contains(payload, "123") {
		t.Fatalf("expected request payload to be included: %s", payload)
	}
}

func TestExtractWebhookURLVariants(t *testing.T) {
	cases := []map[string]any{
		{"webhookURL": "http://example.com"},
		{"webhook_url": "http://example.com"},
	}

	for _, event := range cases {
		url, err := extractWebhookURL(event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "http://example.com" {
			t.Fatalf("unexpected url: %s", url)
		}
	}
}
