package adapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"lambda-to-discord/domain"
)

type CloudWatchSNSAdapter struct {
	webhookURL string
}

func NewCloudWatchSNSAdapter(webhookURL string) CloudWatchSNSAdapter {
	return CloudWatchSNSAdapter{webhookURL: strings.TrimSpace(webhookURL)}
}

func (a CloudWatchSNSAdapter) Transform(event json.RawMessage) (domain.NotificationPayload, map[string]any, error) {
	if strings.TrimSpace(a.webhookURL) == "" {
		return domain.NotificationPayload{}, nil, errors.New("cloudwatch adapter requires webhook url")
	}

	message, err := extractAlarmMessage(event)
	if err != nil {
		return domain.NotificationPayload{}, nil, err
	}

	alarm, err := decodeAlarm(message)
	if err != nil {
		return domain.NotificationPayload{}, nil, err
	}

	payload := domain.NotificationPayload{
		WebhookURL:      a.webhookURL,
		Content:         buildAlarmSummary(alarm),
		AllowedMentions: domain.NoMentions(),
	}

	embed := domain.Embed{
		Title:       alarm.AlarmName,
		Description: buildAlarmDescription(alarm),
		Fields:      buildAlarmFields(alarm),
		Timestamp:   alarm.StateChangeTime,
	}
	switch strings.ToUpper(strings.TrimSpace(alarm.NewStateValue)) {
	case "ALARM":
		embed.Color = 0xE74C3C
	case "OK":
		embed.Color = 0x2ECC71
	}
	if desc := strings.TrimSpace(alarm.AlarmDescription); desc != "" {
		embed.Footer = &domain.EmbedFooter{Text: desc}
	}
	payload.Embeds = append(payload.Embeds, embed)

	var eventMap map[string]any
	if err := json.Unmarshal(message, &eventMap); err != nil {
		eventMap = map[string]any{"raw": string(message)}
	}

	return payload, eventMap, nil
}

type snsEnvelope struct {
	Records []struct {
		Sns struct {
			Message string `json:"Message"`
		} `json:"Sns"`
	} `json:"Records"`
}

type cloudWatchAlarm struct {
	AlarmName        string            `json:"AlarmName"`
	AlarmDescription string            `json:"AlarmDescription"`
	AWSAccountID     string            `json:"AWSAccountId"`
	NewStateValue    string            `json:"NewStateValue"`
	NewStateReason   string            `json:"NewStateReason"`
	StateChangeTime  string            `json:"StateChangeTime"`
	Region           string            `json:"Region"`
	AlarmArn         string            `json:"AlarmArn"`
	OldStateValue    string            `json:"OldStateValue"`
	Trigger          cloudWatchTrigger `json:"Trigger"`
}

type cloudWatchTrigger struct {
	MetricName         string                `json:"MetricName"`
	Namespace          string                `json:"Namespace"`
	StatisticType      string                `json:"StatisticType"`
	Statistic          string                `json:"Statistic"`
	Period             int                   `json:"Period"`
	EvaluationPeriods  int                   `json:"EvaluationPeriods"`
	ComparisonOperator string                `json:"ComparisonOperator"`
	Threshold          json.Number           `json:"Threshold"`
	TreatMissingData   string                `json:"TreatMissingData"`
	Dimensions         []cloudWatchDimension `json:"Dimensions"`
}

type cloudWatchDimension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func extractAlarmMessage(event json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(event)
	if len(trimmed) == 0 {
		return nil, errors.New("cloudwatch alarm message is empty")
	}

	var asString string
	if err := json.Unmarshal(trimmed, &asString); err == nil {
		return json.RawMessage(strings.TrimSpace(asString)), nil
	}

	var envelope snsEnvelope
	if err := json.Unmarshal(trimmed, &envelope); err == nil {
		if len(envelope.Records) > 0 && envelope.Records[0].Sns.Message != "" {
			return json.RawMessage(envelope.Records[0].Sns.Message), nil
		}
	}

	return trimmed, nil
}

func decodeAlarm(raw json.RawMessage) (cloudWatchAlarm, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return cloudWatchAlarm{}, errors.New("cloudwatch alarm payload is empty")
	}

	var alarm cloudWatchAlarm
	if err := json.Unmarshal(trimmed, &alarm); err != nil {
		return cloudWatchAlarm{}, fmt.Errorf("failed to decode cloudwatch alarm: %w", err)
	}

	return alarm, nil
}

func buildAlarmSummary(alarm cloudWatchAlarm) string {
	state := strings.ToLower(strings.TrimSpace(alarm.NewStateValue))
	if state == "" {
		state = "unknown"
	}
	return fmt.Sprintf(":rotating_light: CloudWatch alarm %q is %s", alarm.AlarmName, state)
}

func buildAlarmDescription(alarm cloudWatchAlarm) string {
	if desc := strings.TrimSpace(alarm.NewStateReason); desc != "" {
		return desc
	}
	return "CloudWatch reported a state change."
}

func buildAlarmFields(alarm cloudWatchAlarm) []domain.EmbedField {
	var fields []domain.EmbedField
	appendField := func(name, value string, inline bool) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		fields = append(fields, domain.EmbedField{Name: name, Value: value, Inline: inline})
	}

	appendField("Account", alarm.AWSAccountID, true)
	appendField("Region", alarm.Region, true)
	appendField("Old State", alarm.OldStateValue, true)
	appendField("New State", alarm.NewStateValue, true)
	appendField("Alarm ARN", alarm.AlarmArn, false)

	if metric := buildMetricSummary(alarm.Trigger); metric != "" {
		appendField("Trigger", metric, false)
	}
	if dimensions := buildDimensionsSummary(alarm.Trigger.Dimensions); dimensions != "" {
		appendField("Dimensions", dimensions, false)
	}

	return fields
}

func buildMetricSummary(trigger cloudWatchTrigger) string {
	parts := []string{}
	if trigger.Namespace != "" && trigger.MetricName != "" {
		parts = append(parts, fmt.Sprintf("%s/%s", trigger.Namespace, trigger.MetricName))
	} else if trigger.MetricName != "" {
		parts = append(parts, trigger.MetricName)
	}
	if trigger.StatisticType != "" {
		parts = append(parts, trigger.StatisticType)
	} else if trigger.Statistic != "" {
		parts = append(parts, trigger.Statistic)
	}
	threshold := strings.TrimSpace(trigger.Threshold.String())
	if trigger.ComparisonOperator != "" && threshold != "" {
		parts = append(parts, fmt.Sprintf("%s %s", trigger.ComparisonOperator, threshold))
	}
	if trigger.EvaluationPeriods > 0 {
		parts = append(parts, fmt.Sprintf("for %d periods", trigger.EvaluationPeriods))
	}
	if trigger.Period > 0 {
		parts = append(parts, fmt.Sprintf("period: %ds", trigger.Period))
	}
	if trigger.TreatMissingData != "" {
		parts = append(parts, fmt.Sprintf("missing data: %s", trigger.TreatMissingData))
	}
	return strings.Join(parts, " · ")
}

func buildDimensionsSummary(dimensions []cloudWatchDimension) string {
	if len(dimensions) == 0 {
		return ""
	}
	var parts []string
	for _, dim := range dimensions {
		name := strings.TrimSpace(dim.Name)
		value := strings.TrimSpace(dim.Value)
		if name == "" || value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", name, value))
	}
	return strings.Join(parts, ", ")
}
