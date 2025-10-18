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
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	return s.resp, nil
}

func TestHandleRequestDirectSuccess(t *testing.T) {
	t.Setenv(adapterTypeEnvVar, "direct")
	stub := &stubHTTPClient{resp: &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader("ok"))}}
	oldClient := defaultHTTPClient
	defaultHTTPClient = stub
	t.Cleanup(func() { defaultHTTPClient = oldClient })

	event := map[string]any{
		"webhookURL": "https://discord.example/direct",
		"content":    "hello",
	}
	raw, _ := json.Marshal(event)

	resp, err := HandleRequest(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	if stub.req == nil {
		t.Fatalf("expected request to be dispatched")
	}
}

func TestHandleRequestCloudWatchSuccess(t *testing.T) {
	t.Setenv(adapterTypeEnvVar, "cloudwatch")
	t.Setenv(cloudWatchWebhookEnvVar, "https://discord.example/cloudwatch")
	stub := &stubHTTPClient{}
	oldClient := defaultHTTPClient
	defaultHTTPClient = stub
	t.Cleanup(func() { defaultHTTPClient = oldClient })

	raw := json.RawMessage(sampleAlarmMessage)
	resp, err := HandleRequest(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestHandleRequestNotifiesOnError(t *testing.T) {
	t.Setenv(adapterTypeEnvVar, "direct")
	t.Setenv(errorWebhookEnvVar, "https://discord.example/error")

	stub := &stubHTTPClient{}
	oldClient := defaultHTTPClient
	defaultHTTPClient = stub
	t.Cleanup(func() { defaultHTTPClient = oldClient })

	_, err := HandleRequest(context.Background(), json.RawMessage(`123`))
	if err == nil {
		t.Fatal("expected error")
	}
	if stub.req == nil {
		t.Fatal("expected error notification to be sent")
	}
	if stub.req.URL.String() != "https://discord.example/error" {
		t.Fatalf("unexpected error webhook: %s", stub.req.URL.String())
	}
	body, readErr := io.ReadAll(stub.req.Body)
	if readErr != nil {
		t.Fatalf("failed to read body: %v", readErr)
	}
	if !strings.Contains(string(body), "Failed to process request") {
		t.Fatalf("expected error message in payload: %s", string(body))
	}
}

func TestBuildNotificationPayloadUnsupported(t *testing.T) {
	if _, _, err := buildNotificationPayload("unknown", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected unsupported adapter error")
	}
}

func TestBuildErrorNotificationPayload(t *testing.T) {
	payload := buildErrorNotificationPayload("https://discord.example/error", json.RawMessage(`{"foo":"bar"}`), map[string]any{"foo": "bar"}, errors.New("boom"))
	if payload.WebhookURL != "https://discord.example/error" {
		t.Fatalf("unexpected webhook: %s", payload.WebhookURL)
	}
	if payload.AllowedMentions == nil || len(payload.AllowedMentions.Parse) != 0 {
		t.Fatalf("expected mentions to be disabled")
	}
	if len(payload.Embeds) != 1 {
		t.Fatalf("expected embed to be present")
	}
}

const sampleAlarmMessage = `{
  "AlarmName": "CPUHigh",
  "AlarmDescription": "CPU usage is high",
  "AWSAccountId": "123456789012",
  "NewStateValue": "ALARM",
  "NewStateReason": "Threshold Crossed",
  "StateChangeTime": "2024-01-02T03:04:05.678Z",
  "Region": "us-east-1",
  "AlarmArn": "arn:aws:cloudwatch:us-east-1:123456789012:alarm:CPUHigh",
  "OldStateValue": "OK",
  "Trigger": {
    "MetricName": "CPUUtilization",
    "Namespace": "AWS/EC2",
    "StatisticType": "Statistic",
    "Statistic": "AVERAGE",
    "Period": 60,
    "EvaluationPeriods": 1,
    "ComparisonOperator": "GreaterThanThreshold",
    "Threshold": "80",
    "TreatMissingData": "missing",
    "Dimensions": [
      {"name": "InstanceId", "value": "i-1234567890abcdef0"}
    ]
  }
}`
