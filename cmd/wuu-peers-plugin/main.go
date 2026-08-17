package main

import (
	"context"
	"log"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/peers"
)

func main() {
	if err := pluginapi.Serve(context.Background(), peers.Handler()); err != nil {
		log.Fatal(err)
	}
}
