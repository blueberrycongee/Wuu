package main

import (
	"context"
	"log"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/plan"
)

func main() {
	if err := pluginapi.Serve(context.Background(), plan.Handler()); err != nil {
		log.Fatal(err)
	}
}
