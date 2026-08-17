package main

import (
	"context"
	"fmt"
	"os"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/notecompaction"
)

func main() {
	if err := pluginapi.Serve(context.Background(), notecompaction.Handler()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
