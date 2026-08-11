package subagent_test

import (
	"context"
	"os"
	"testing"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/subagent"
)

func TestSubagentPluginProcessHelper(t *testing.T) {
	if os.Getenv("WUU_SUBAGENT_PLUGIN_TEST_HELPER") != "1" {
		return
	}
	if err := pluginapi.Serve(context.Background(), subagent.Handler()); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
