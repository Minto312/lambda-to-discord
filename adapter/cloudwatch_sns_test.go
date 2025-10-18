package adapter

import (
	"encoding/json"
	"testing"
)

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

func TestCloudWatchSNSAdapterTransform(t *testing.T) {
	raw := json.RawMessage(sampleAlarmMessage)
	adapter := NewCloudWatchSNSAdapter("https://discord.example/cloudwatch")

	payload, eventMap, err := adapter.Transform(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.WebhookURL != "https://discord.example/cloudwatch" {
		t.Fatalf("unexpected webhook: %s", payload.WebhookURL)
	}
	if payload.AllowedMentions == nil || len(payload.AllowedMentions.Parse) != 0 {
		t.Fatalf("expected mentions to be disabled: %#v", payload.AllowedMentions)
	}
	if len(payload.Embeds) != 1 {
		t.Fatalf("expected a single embed, got %d", len(payload.Embeds))
	}
	embed := payload.Embeds[0]
	if embed.Title != "CPUHigh" {
		t.Fatalf("unexpected title: %s", embed.Title)
	}
	if len(embed.Fields) == 0 {
		t.Fatalf("expected embed fields to be populated")
	}
	if eventMap["AlarmName"].(string) != "CPUHigh" {
		t.Fatalf("expected event map to contain alarm: %#v", eventMap)
	}
}

func TestCloudWatchSNSAdapterTransformEnvelope(t *testing.T) {
	envelope := json.RawMessage(`{"Records":[{"Sns":{"Message":` + sampleAlarmMessage + `}}]}`)
	adapter := NewCloudWatchSNSAdapter("https://discord.example/cloudwatch")
	payload, _, err := adapter.Transform(envelope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.Content == "" {
		t.Fatalf("expected content to be populated")
	}
}

func TestCloudWatchSNSAdapterErrors(t *testing.T) {
	if _, _, err := NewCloudWatchSNSAdapter("").Transform(json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error when webhook missing")
	}
	if _, _, err := NewCloudWatchSNSAdapter("https://hook").Transform(json.RawMessage(`""`)); err == nil {
		t.Fatal("expected error for empty message")
	}
}
