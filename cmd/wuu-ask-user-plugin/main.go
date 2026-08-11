package main

import (
	"context"
	"log"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	askuser "github.com/blueberrycongee/wuu/plugins/ask-user"
)

func main() {
	if err := pluginapi.Serve(context.Background(), askuser.Handler()); err != nil {
		log.Fatal(err)
	}
}
