package main

import (
	"context"
	"log"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/automation"
)

func main() {
	if err := pluginapi.Serve(context.Background(), automation.Handler()); err != nil {
		log.Fatal(err)
	}
}
