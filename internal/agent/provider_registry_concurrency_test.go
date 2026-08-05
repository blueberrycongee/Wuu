package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/blueberrycongee/wuu/internal/providers"
)

func TestPluginProviderRegistriesAllowConcurrentReloadAndResolution(t *testing.T) {
	compactions := NewCompactionRegistry()
	models := NewModelProviderRegistry()

	const workers = 16
	const iterations = 100
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := 0; iteration < iterations; iteration++ {
				key := fmt.Sprintf("plugin-%d", worker)
				compactions.Register(concurrentCompactionProvider{key: key})
				models.Register(concurrentModelProvider{key: key})
				_ = compactions.Resolve(nil)
				_ = models.Resolve("test-model")
				_ = compactions.Count()
				_ = models.Count()
				compactions.Unregister(key)
				models.Unregister(key)
			}
		}()
	}
	wg.Wait()
}

type concurrentCompactionProvider struct{ key string }

func (p concurrentCompactionProvider) CompactionKey() string { return p.key }
func (concurrentCompactionProvider) Compact(_ context.Context, _ string, messages []providers.ChatMessage) ([]providers.ChatMessage, error) {
	return messages, nil
}
func (concurrentCompactionProvider) CompactionPriority() int { return 0 }

type concurrentModelProvider struct{ key string }

func (p concurrentModelProvider) ProviderKey() string     { return p.key }
func (concurrentModelProvider) SupportsModel(string) bool { return true }
func (concurrentModelProvider) CreateClient(context.Context, string, ModelProviderOptions) (providers.Client, error) {
	return nil, nil
}
func (concurrentModelProvider) Priority() int { return 0 }
