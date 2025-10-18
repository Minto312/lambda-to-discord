package discord

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"lambda-to-discord/domain"
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
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	return s.resp, nil
}

func TestSendSuccess(t *testing.T) {
	stub := &stubHTTPClient{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}}
	payload := domain.NotificationPayload{WebhookURL: "https://discord.example/hook", Content: "hello"}

	status, body, err := Send(context.Background(), stub, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("unexpected status: %d", status)
	}
	if body != "ok" {
		t.Fatalf("unexpected body: %s", body)
	}
	if stub.req == nil {
		t.Fatalf("expected request to be sent")
	}
	if stub.req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected content type: %s", stub.req.Header.Get("Content-Type"))
	}
}

func TestSendNetworkError(t *testing.T) {
	stub := &stubHTTPClient{err: errors.New("boom")}
	payload := domain.NotificationPayload{WebhookURL: "https://discord.example/hook", Content: "hello"}
	_, _, err := Send(context.Background(), stub, payload)
	if err == nil {
		t.Fatal("expected error")
	}
	var webhookErr *WebhookError
	if !errors.As(err, &webhookErr) {
		t.Fatalf("expected webhook error, got %T", err)
	}
}

func TestSendValidation(t *testing.T) {
	if _, _, err := Send(context.Background(), &stubHTTPClient{}, domain.NotificationPayload{}); err == nil {
		t.Fatal("expected validation error")
	}
}
