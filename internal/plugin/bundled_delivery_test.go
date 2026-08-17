package plugin

import "testing"

func TestBundledDeliveryResolvesDesktopPresenter(t *testing.T) {
	plugins := discoverBundled(t.TempDir(), DiscoverOptions{GOOS: "darwin", LookupEnv: func(string) (string, bool) {
		return "", false
	}})
	for _, item := range plugins {
		if item.ID != "delivery" {
			continue
		}
		if item.Desktop == nil || item.Desktop.Entry != "desktop.js" {
			t.Fatalf("delivery desktop = %+v", item.Desktop)
		}
		if item.Runtime != nil {
			t.Fatalf("delivery should be desktop-only, got runtime = %+v", item.Runtime)
		}
		if len(item.Presenters) != 1 || item.Presenters[0].ID != "delivery.inspect" || item.Presenters[0].Target != "conversation.item" || string(item.Presenters[0].Mode) != "wrap" {
			t.Fatalf("delivery presenters = %+v", item.Presenters)
		}
		return
	}
	t.Fatal("bundled delivery plugin was not discovered")
}
