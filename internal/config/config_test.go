package config

// Tests for settings load/save and key validation. No test
// file existed in this package before; these are the first.

import (
	"testing"
)

func TestLoadSettingsEmptyHomeReturnsDefaults(t *testing.T) {
	settings, err := LoadSettings(t.TempDir())
	if err != nil {
		t.Fatalf("LoadSettings on empty home: %v", err)
	}
	if settings.DailyQuotaBudget != 10000 {
		t.Errorf("DailyQuotaBudget = %d, want 10000", settings.DailyQuotaBudget)
	}
	if settings.ServerAddr != ":8765" {
		t.Errorf("ServerAddr = %q, want :8765", settings.ServerAddr)
	}
	if settings.ReadOnly {
		t.Error("ReadOnly default is true, want false")
	}
}

func TestSaveThenLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	settings, err := LoadSettings(home)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	settings.DailyQuotaBudget = 4242
	settings.ReadOnly = true
	if err := settings.Save(home); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := LoadSettings(home)
	if err != nil {
		t.Fatalf("LoadSettings after Save: %v", err)
	}
	if loaded.DailyQuotaBudget != 4242 {
		t.Errorf("DailyQuotaBudget = %d, want 4242", loaded.DailyQuotaBudget)
	}
	if !loaded.ReadOnly {
		t.Error("ReadOnly = false after round-trip, want true")
	}
}

func TestSetByKey(t *testing.T) {
	settings := DefaultSettings()

	if err := settings.SetByKey("no_such_key", "1"); err == nil {
		t.Error("SetByKey unknown key: want an error, got nil")
	}
	if err := settings.SetByKey("daily_quota_budget", "2500"); err != nil {
		t.Errorf("SetByKey daily_quota_budget: %v", err)
	}
	if settings.DailyQuotaBudget != 2500 {
		t.Errorf("DailyQuotaBudget = %d, want 2500", settings.DailyQuotaBudget)
	}
	if err := settings.SetByKey("daily_quota_budget", "not-a-number"); err == nil {
		t.Error("SetByKey daily_quota_budget with garbage: want an error, got nil")
	}
	if err := settings.SetByKey("read_only", "true"); err != nil {
		t.Errorf("SetByKey read_only: %v", err)
	}
	if !settings.ReadOnly {
		t.Error("ReadOnly = false after SetByKey true, want true")
	}
}
