package main

import (
	"context"
	"log"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/memory"
)

func main() {
	if err := pluginapi.Serve(context.Background(), memory.Handler()); err != nil {
		log.Fatal(err)
	}
}
