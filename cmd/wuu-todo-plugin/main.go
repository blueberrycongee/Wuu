package main

import (
	"context"
	"log"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/todo"
)

func main() {
	if err := pluginapi.Serve(context.Background(), todo.Handler()); err != nil {
		log.Fatal(err)
	}
}
