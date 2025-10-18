package domain

import (
	"errors"
	"strings"
)

type NotificationPayload struct {
	WebhookURL      string
	Content         string
	Embeds          []Embed
	AllowedMentions *AllowedMentions
	Username        string
	AvatarURL       string
}

type Embed struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color,omitempty"`
	Fields      []EmbedField `json:"fields,omitempty"`
	Footer      *EmbedFooter `json:"footer,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
}

type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type EmbedFooter struct {
	Text string `json:"text"`
}

type AllowedMentions struct {
	Parse       []string `json:"parse,omitempty"`
	Users       []string `json:"users,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	RepliedUser bool     `json:"replied_user,omitempty"`
}

func (a *AllowedMentions) IsZero() bool {
	if a == nil {
		return true
	}
	return len(a.Parse) == 0 && len(a.Users) == 0 && len(a.Roles) == 0 && !a.RepliedUser
}

func NoMentions() *AllowedMentions {
	return &AllowedMentions{Parse: []string{}}
}

func (p NotificationPayload) Validate() error {
	if strings.TrimSpace(p.WebhookURL) == "" {
		return errors.New("discord webhook URL must be provided")
	}
	if strings.TrimSpace(p.Content) == "" && len(p.Embeds) == 0 {
		return errors.New("notification payload must include content or embeds")
	}
	return nil
}
