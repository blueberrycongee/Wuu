package providerfactory

import (
	"os"
	"testing"

	"github.com/blueberrycongee/wuu/internal/config"
)

func TestBuildClient_OpenAICompatible(t *testing.T) {
	t.Setenv("TEST_WUU_KEY", "abc")

	client, err := BuildClient(config.ProviderConfig{
		Type:      "openai-compatible",
		BaseURL:   "https://example.com/v1",
		APIKeyEnv: "TEST_WUU_KEY",
		Model:     "gpt-test",
	})
	if err != nil {
		t.Fatalf("BuildClient returned error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
}

func TestBuildClient_Anthropic(t *testing.T) {
	t.Setenv("TEST_ANTHROPIC_KEY", "abc")

	client, err := BuildClient(config.ProviderConfig{
		Type:      "anthropic",
		BaseURL:   "https://api.anthropic.com",
		APIKeyEnv: "TEST_ANTHROPIC_KEY",
		Model:     "claude-test",
	})
	if err != nil {
		t.Fatalf("BuildClient returned error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
}

func TestResolveAPIKey_AuthStoreFallback(t *testing.T) {
	// Clear default env var so fallback to auth store is exercised.
	t.Setenv("OPENAI_API_KEY", "")

	home := t.TempDir()
	// Save key to auth store.
	if err := config.SaveAuthKey(home, "myapi", "sk-from-auth-store"); err != nil {
		t.Fatalf("save auth key: %v", err)
	}

	provider := config.ProviderConfig{
		Type:    "openai-compatible",
		BaseURL: "https://example.com/v1",
		Model:   "test",
		// No APIKey, no APIKeyEnv set.
	}

	key, err := ResolveAPIKeyWithHome(provider, "myapi", home)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if key != "sk-from-auth-store" {
		t.Fatalf("expected sk-from-auth-store, got %q", key)
	}
}

func TestBuildClient_MissingAPIKey(t *testing.T) {
	_ = os.Unsetenv("MISSING_WUU_KEY")

	_, err := BuildClient(config.ProviderConfig{
		Type:      "openai-compatible",
		BaseURL:   "https://example.com/v1",
		APIKeyEnv: "MISSING_WUU_KEY",
		Model:     "gpt-test",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
