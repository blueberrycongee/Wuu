package providerfactory

import (
	"fmt"
	"os"
	"strings"

	"github.com/blueberrycongee/wuu/internal/config"
	"github.com/blueberrycongee/wuu/internal/providers"
	"github.com/blueberrycongee/wuu/internal/providers/anthropic"
	"github.com/blueberrycongee/wuu/internal/providers/openai"
)

// BuildClient constructs a provider client from config.
func BuildClient(provider config.ProviderConfig) (providers.Client, error) {
	typeName := normalizeType(provider.Type)
	apiKey, err := resolveAPIKey(provider)
	if err != nil {
		return nil, err
	}

	switch typeName {
	case "openai", "openai-compatible", "codex":
		client, newErr := openai.New(openai.ClientConfig{
			BaseURL: provider.BaseURL,
			APIKey:  apiKey,
			Headers: provider.Headers,
		})
		if newErr != nil {
			return nil, newErr
		}
		return client, nil
	case "anthropic", "claude", "anthropic-official":
		client, newErr := anthropic.New(anthropic.ClientConfig{
			BaseURL: provider.BaseURL,
			APIKey:  apiKey,
			Headers: provider.Headers,
		})
		if newErr != nil {
			return nil, newErr
		}
		return client, nil
	default:
		return nil, fmt.Errorf("unsupported provider type %q", provider.Type)
	}
}

// BuildStreamClient constructs a streaming-capable provider client.
func BuildStreamClient(provider config.ProviderConfig) (providers.StreamClient, error) {
	typeName := normalizeType(provider.Type)
	apiKey, err := resolveAPIKey(provider)
	if err != nil {
		return nil, err
	}

	switch typeName {
	case "openai", "openai-compatible", "codex":
		return openai.New(openai.ClientConfig{
			BaseURL: provider.BaseURL,
			APIKey:  apiKey,
			Headers: provider.Headers,
		})
	case "anthropic", "claude", "anthropic-official":
		return anthropic.New(anthropic.ClientConfig{
			BaseURL: provider.BaseURL,
			APIKey:  apiKey,
			Headers: provider.Headers,
		})
	default:
		return nil, fmt.Errorf("unsupported provider type %q", provider.Type)
	}
}

func normalizeType(value string) string {
	s := strings.ToLower(strings.TrimSpace(value))
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

// ResolveAPIKeyWithHome resolves API key with explicit home directory.
func ResolveAPIKeyWithHome(provider config.ProviderConfig, providerName, home string) (string, error) {
	// 1. Explicit api_key in config.
	if strings.TrimSpace(provider.APIKey) != "" {
		return strings.TrimSpace(provider.APIKey), nil
	}

	// 2. Environment variable.
	envKey := strings.TrimSpace(provider.APIKeyEnv)
	if envKey == "" {
		envKey = defaultAPIKeyEnv(normalizeType(provider.Type))
	}
	if envKey != "" {
		value := strings.TrimSpace(os.Getenv(envKey))
		if value != "" {
			return value, nil
		}
	}

	// 3. Global auth store.
	if home != "" && providerName != "" {
		key, err := config.LoadAuthKey(home, providerName)
		if err == nil && key != "" {
			return key, nil
		}
	}

	hint := "set api_key or run wuu init"
	if envKey != "" {
		hint = fmt.Sprintf("set api_key, %s env var, or run wuu init", envKey)
	}
	return "", fmt.Errorf("no API key found for provider %q (%s)", provider.Type, hint)
}

func resolveAPIKey(provider config.ProviderConfig) (string, error) {
	return ResolveAPIKeyWithHome(provider, "", os.Getenv("HOME"))
}

func defaultAPIKeyEnv(providerType string) string {
	switch providerType {
	case "openai", "openai-compatible", "codex":
		return "OPENAI_API_KEY"
	case "anthropic", "claude", "anthropic-official":
		return "ANTHROPIC_API_KEY"
	default:
		return ""
	}
}
