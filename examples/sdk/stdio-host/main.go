// stdio-host embeds the Wuu core and exposes its app-server protocol over the
// process standard streams. Replace the streams with pipes, sockets, or another
// framed transport when integrating a custom shell.
package main

import (
	"context"
	"log"
	"os"

	"github.com/blueberrycongee/wuu/sdk"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	runtime, err := sdk.New(sdk.Options{
		WorkDir:               os.Getenv("WUU_WORKDIR"),
		CreateConfigIfMissing: true,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := runtime.Close(); err != nil {
			log.Printf("close Wuu runtime: %v", err)
		}
	}()

	return runtime.Serve(context.Background(), os.Stdin, os.Stdout)
}
