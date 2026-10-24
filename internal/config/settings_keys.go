package config

import (
	"fmt"
	"strconv"
	"strings"
)

// SettingKey describes one user-facing setting: its stable key
// (shared by the Settings UI, the API, and the `settings` CLI
// group - spec 29, item 60), where it lives in Settings, and how
// it is validated.
type SettingKey struct {
	Key     string
	Label   string
	Group   string
	Type    string // "int", "string", "port", "secret"
	Default string
}

// SettingKeys is the canonical list of user-facing settings.
func SettingKeys() []SettingKey {
	return []SettingKey{
		{Key: "daily_quota_budget", Label: "Daily API quota budget", Group: "General", Type: "int", Default: "10000"},
		{Key: "server_addr", Label: "UI / API listen address", Group: "General", Type: "string", Default: ":8765"},
		{Key: "default_account", Label: "Default account (CLI)", Group: "General", Type: "int", Default: "0"},
		{Key: "default_channel", Label: "Default channel (CLI)", Group: "General", Type: "int", Default: "0"},
		{Key: "read_only", Label: "Read-only mode (block all writes)", Group: "General", Type: "bool", Default: "false"},
		{Key: "quiet_hours_start", Label: "Quiet hours start (0-23, = end disables)", Group: "Automation", Type: "int", Default: "0"},
		{Key: "quiet_hours_end", Label: "Quiet hours end (0-23)", Group: "Automation", Type: "int", Default: "0"},
		{Key: "webhook_url", Label: "Webhook URL (events POST here)", Group: "Automation", Type: "string", Default: ""},
		{Key: "webhook_secret", Label: "Webhook signing secret", Group: "Automation", Type: "secret", Default: ""},
		{Key: "mail.smtp_host", Label: "SMTP host", Group: "Mail", Type: "string", Default: ""},
		{Key: "mail.smtp_port", Label: "SMTP port", Group: "Mail", Type: "port", Default: "0"},
		{Key: "mail.imap_host", Label: "IMAP host", Group: "Mail", Type: "string", Default: ""},
		{Key: "mail.imap_port", Label: "IMAP port", Group: "Mail", Type: "port", Default: "0"},
		{Key: "mail.username", Label: "Mail username", Group: "Mail", Type: "string", Default: ""},
		{Key: "mail.password", Label: "Mail password", Group: "Mail", Type: "secret", Default: ""},
	}
}

// FindSettingKey looks up a setting key by name.
func FindSettingKey(key string) (SettingKey, bool) {
	for _, candidate := range SettingKeys() {
		if candidate.Key == key {
			return candidate, true
		}
	}
	return SettingKey{}, false
}

// GetByKey returns the current value of a setting. Secret values
// are never returned; a secret reports whether one is set.
func (s *Settings) GetByKey(key string) (string, bool) {
	switch key {
	case "daily_quota_budget":
		return strconv.Itoa(s.DailyQuotaBudget), true
	case "server_addr":
		return s.ServerAddr, true
	case "default_account":
		return strconv.Itoa(s.DefaultAccount), true
	case "default_channel":
		return strconv.Itoa(s.DefaultChannel), true
	case "read_only":
		return strconv.FormatBool(s.ReadOnly), true
	case "quiet_hours_start":
		return strconv.Itoa(s.QuietHoursStart), true
	case "quiet_hours_end":
		return strconv.Itoa(s.QuietHoursEnd), true
	case "webhook_url":
		return s.WebhookURL, true
	case "webhook_secret":
		if s.WebhookSecret != "" {
			return "set", true
		}
		return "", true
	case "mail.smtp_host":
		return s.Mail.SMTPHost, true
	case "mail.smtp_port":
		return strconv.Itoa(s.Mail.SMTPPort), true
	case "mail.imap_host":
		return s.Mail.IMAPHost, true
	case "mail.imap_port":
		return strconv.Itoa(s.Mail.IMAPPort), true
	case "mail.username":
		return s.Mail.Username, true
	case "mail.password":
		if s.Mail.Password != "" {
			return "set", true
		}
		return "", true
	}
	return "", false
}

// SetByKey validates and applies one setting value. Validation
// happens before anything is stored (spec 29, item 59): an invalid
// value returns an error and leaves the settings untouched.
func (s *Settings) SetByKey(key string, value string) error {
	def, found := FindSettingKey(key)
	if !found {
		return fmt.Errorf("unknown setting %q", key)
	}
	switch def.Type {
	case "int":
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || parsed < 0 || parsed > 1000000 {
			return fmt.Errorf("%s must be a whole number between 0 and 1000000", def.Label)
		}
	case "port":
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || parsed < 0 || parsed > 65535 {
			return fmt.Errorf("%s must be a port number between 0 and 65535", def.Label)
		}
	case "string":
		if key == "server_addr" && !strings.Contains(value, ":") {
			return fmt.Errorf("%s must look like host:port (for example :8765)", def.Label)
		}
	case "bool":
		if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
			return fmt.Errorf("%s must be true or false", def.Label)
		}
	}
	if key == "quiet_hours_start" || key == "quiet_hours_end" {
		parsed, _ := strconv.Atoi(strings.TrimSpace(value))
		if parsed < 0 || parsed > 23 {
			return fmt.Errorf("%s must be an hour between 0 and 23", def.Label)
		}
	}
	if key == "webhook_url" && value != "" && !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return fmt.Errorf("%s must start with http:// or https://", def.Label)
	}
	switch key {
	case "daily_quota_budget":
		parsed, _ := strconv.Atoi(strings.TrimSpace(value))
		s.DailyQuotaBudget = parsed
	case "server_addr":
		s.ServerAddr = value
	case "default_account":
		parsed, _ := strconv.Atoi(strings.TrimSpace(value))
		s.DefaultAccount = parsed
	case "default_channel":
		parsed, _ := strconv.Atoi(strings.TrimSpace(value))
		s.DefaultChannel = parsed
	case "read_only":
		parsed, _ := strconv.ParseBool(strings.TrimSpace(value))
		s.ReadOnly = parsed
	case "quiet_hours_start":
		parsed, _ := strconv.Atoi(strings.TrimSpace(value))
		s.QuietHoursStart = parsed
	case "quiet_hours_end":
		parsed, _ := strconv.Atoi(strings.TrimSpace(value))
		s.QuietHoursEnd = parsed
	case "webhook_url":
		s.WebhookURL = value
	case "webhook_secret":
		s.WebhookSecret = value
	case "mail.smtp_host":
		s.Mail.SMTPHost = value
	case "mail.smtp_port":
		parsed, _ := strconv.Atoi(strings.TrimSpace(value))
		s.Mail.SMTPPort = parsed
	case "mail.imap_host":
		s.Mail.IMAPHost = value
	case "mail.imap_port":
		parsed, _ := strconv.Atoi(strings.TrimSpace(value))
		s.Mail.IMAPPort = parsed
	case "mail.username":
		s.Mail.Username = value
	case "mail.password":
		s.Mail.Password = value
	}
	return nil
}

// Snapshot returns the settings as a key/value map. Secrets are
// excluded and reported in the second result (spec 29, item 57).
func (s *Settings) Snapshot() (map[string]string, []string) {
	values := make(map[string]string)
	excluded := []string{}
	for _, def := range SettingKeys() {
		if def.Type == "secret" {
			value, _ := s.GetByKey(def.Key)
			if value == "set" {
				excluded = append(excluded, def.Key)
			}
			continue
		}
		value, _ := s.GetByKey(def.Key)
		values[def.Key] = value
	}
	return values, excluded
}
