package tui

import "testing"

func TestNormalizeThemeMode(t *testing.T) {
	tests := []struct {
		in   string
		want themeMode
	}{
		{"", themeModeAuto},
		{"auto", themeModeAuto},
		{"Auto", themeModeAuto},
		{"dark", themeModeDark},
		{"light", themeModeLight},
		{"invalid", ""},
	}

	for _, tc := range tests {
		if got := normalizeThemeMode(tc.in); got != tc.want {
			t.Fatalf("normalizeThemeMode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSetThemeMode_Explicit(t *testing.T) {
	prev := currentTheme
	t.Cleanup(func() {
		applyTheme(prev)
	})

	if err := SetThemeMode("light"); err != nil {
		t.Fatalf("SetThemeMode(light): %v", err)
	}
	if currentTheme.Brand != lightTheme.Brand {
		t.Fatalf("expected light theme brand, got %v", currentTheme.Brand)
	}
	if currentTheme.UserBubbleBg != lightTheme.UserBubbleBg {
		t.Fatalf("expected light user bubble bg, got %v", currentTheme.UserBubbleBg)
	}

	if err := SetThemeMode("dark"); err != nil {
		t.Fatalf("SetThemeMode(dark): %v", err)
	}
	if currentTheme.Brand != darkTheme.Brand {
		t.Fatalf("expected dark theme brand, got %v", currentTheme.Brand)
	}
	if currentTheme.UserBubbleBg != darkTheme.UserBubbleBg {
		t.Fatalf("expected dark user bubble bg, got %v", currentTheme.UserBubbleBg)
	}
}

func TestSetThemeMode_Auto(t *testing.T) {
	prev := currentTheme
	t.Cleanup(func() {
		applyTheme(prev)
	})

	if err := SetThemeMode("auto"); err != nil {
		t.Fatalf("SetThemeMode(auto): %v", err)
	}
	if currentTheme.Brand != darkTheme.Brand && currentTheme.Brand != lightTheme.Brand {
		t.Fatalf("auto mode selected unknown palette: %v", currentTheme.Brand)
	}
}

func TestSetThemeMode_Invalid(t *testing.T) {
	if err := SetThemeMode("sepia"); err == nil {
		t.Fatal("expected error for invalid theme mode")
	}
}

func TestThemeOptionsIncludesAuto(t *testing.T) {
	opts := themeOptions()
	if len(opts) < 3 {
		t.Fatalf("expected >=3 theme options, got %d", len(opts))
	}
	if opts[0].value != "auto" {
		t.Fatalf("expected first theme option to be auto, got %q", opts[0].value)
	}
}
