// Command agent-session embeds Wuu and drives one agent session directly
// through the public Go SDK. It uses the same app-server lifecycle as Wuu's
// other hosts, including persistence, permissions, plugins, and recovery.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/sdk"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	prompt := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	if prompt == "" {
		return errors.New("usage: go run ./examples/sdk/agent-session <prompt>")
	}

	runtime, err := sdk.New(sdk.Options{
		WorkDir:               os.Getenv("WUU_WORKDIR"),
		CreateConfigIfMissing: true,
	})
	if err != nil {
		return err
	}

	connectionCtx, cancelConnection := context.WithCancel(context.Background())
	client, err := runtime.Connect(connectionCtx, sdk.ClientOptions{Name: "agent-session-example"})
	if err != nil {
		cancelConnection()
		_ = runtime.Close()
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.Close(closeCtx)
		cancelConnection()
		_ = runtime.Close()
	}()

	session, err := client.NewSession(context.Background(), sdk.SessionOptions{})
	if err != nil {
		return err
	}
	subscription := session.Subscribe(connectionCtx, sdk.SubscriptionOptions{Buffer: 64})
	eventsDone := make(chan struct{})
	go func() {
		defer close(eventsDone)
		for event := range subscription.Events {
			fmt.Fprintln(os.Stderr, event.Method)
		}
	}()
	defer func() {
		subscription.Close()
		<-eventsDone
	}()

	run, err := session.Send(context.Background(), sdk.SendOptions{Prompt: prompt})
	if err != nil {
		return err
	}
	result, err := run.Wait(context.Background())
	if err != nil {
		return err
	}
	if result.Run.Status != sdk.RunCompleted {
		if result.Run.Error != nil {
			return fmt.Errorf("run %s: %s", result.Run.Status, result.Run.Error.Message)
		}
		return fmt.Errorf("run ended with status %s", result.Run.Status)
	}
	fmt.Println(result.FinalMessage)
	return nil
}
