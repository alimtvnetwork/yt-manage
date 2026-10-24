// Package config resolves the yt-manage home directory and loads and
// saves the application settings stored in <home>/config.json.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MailSettings holds the SMTP/IMAP connection settings used for
// publish-event mail. The password is stored in the settings file,
// which Save writes with 0600 permissions.
type MailSettings struct {
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	IMAPHost string `json:"imap_host"`
	IMAPPort int    `json:"imap_port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Settings holds the user-configurable application settings.
type Settings struct {
	DailyQuotaBudget int          `json:"daily_quota_budget"`
	ServerAddr       string       `json:"server_addr"`
	DefaultAccount   int          `json:"default_account"`
	DefaultChannel   int          `json:"default_channel"`
	ReadOnly         bool         `json:"read_only"`
	Mail             MailSettings `json:"mail"`
}

// HomeDir returns the yt-manage home directory: $YT_MANAGE_HOME when
// set, otherwise ~/.yt-manage.
func HomeDir() (string, error) {
	if override := os.Getenv("YT_MANAGE_HOME"); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".yt-manage"), nil
}

// DataDir returns the data directory holding the split databases:
// <home>/data.
func DataDir() (string, error) {
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "data"), nil
}

// ClientSecretPath returns the path of the Google OAuth client secret
// file inside the given home directory.
func ClientSecretPath(home string) string {
	return filepath.Join(home, "client_secret.json")
}

// settingsPath returns the path of the settings file inside the given
// home directory.
func settingsPath(home string) string {
	return filepath.Join(home, "config.json")
}

// DefaultSettings returns the built-in settings used when no
// config.json exists.
func DefaultSettings() *Settings {
	return &Settings{
		DailyQuotaBudget: 10000,
		ServerAddr:       ":8765",
	}
}

// LoadSettings reads <home>/config.json. A missing file is not an
// error: the defaults are returned instead.
func LoadSettings(home string) (*Settings, error) {
	data, err := os.ReadFile(settingsPath(home))
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSettings(), nil
		}
		return nil, fmt.Errorf("read settings: %w", err)
	}
	settings := DefaultSettings()
	if err := json.Unmarshal(data, settings); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}
	return settings, nil
}

// Save writes the settings to <home>/config.json with 0600
// permissions, creating the home directory when needed.
func (s *Settings) Save(home string) error {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return fmt.Errorf("create home directory: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := os.WriteFile(settingsPath(home), data, 0o600); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	return nil
}
