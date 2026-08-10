package main

import (
	"context"
	"log"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/singlepass"
)

func main() {
	if err := pluginapi.Serve(context.Background(), singlepass.Handler()); err != nil {
		log.Fatal(err)
	}
}
