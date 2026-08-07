package main

import (
	"context"
	"log"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/goal"
)

func main() {
	if err := pluginapi.Serve(context.Background(), goal.Handler()); err != nil {
		log.Fatal(err)
	}
}
