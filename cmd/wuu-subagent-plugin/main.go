package main

import (
	"context"
	"log"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/subagent"
)

func main() {
	if err := pluginapi.Serve(context.Background(), subagent.Handler()); err != nil {
		log.Fatal(err)
	}
}
