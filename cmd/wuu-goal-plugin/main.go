package main

import (
	"context"
	"fmt"
	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/goal"
	"os"
)

func main() {
	if err := pluginapi.Serve(context.Background(), goal.Handler()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
