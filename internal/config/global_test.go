package config

import "testing"

func TestGlobalConfig_RoundTrip(t *testing.T) {
	home := t.TempDir()
	gc := GlobalConfig{Theme: "dark", HasCompletedOnboarding: true}
	if err := SaveGlobalConfig(home, gc); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadGlobalConfig(home)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Theme != "dark" {
		t.Fatalf("theme mismatch: %q", loaded.Theme)
	}
	if !loaded.HasCompletedOnboarding {
		t.Fatal("expected onboarding completed")
	}
}

func TestGlobalConfig_DefaultsWhenMissing(t *testing.T) {
	home := t.TempDir()
	gc, err := LoadGlobalConfig(home)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if gc.Theme != "" {
		t.Fatalf("expected empty theme, got %q", gc.Theme)
	}
}
