package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/blueberrycongee/wuu/internal/providers/xaisub"
	"github.com/blueberrycongee/wuu/internal/securefs"
	"github.com/blueberrycongee/wuu/internal/statepath"
)

func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	noOpen := fs.Bool("no-open", false, "do not open the verification URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("usage: wuu login xai")
	}
	provider := strings.ToLower(strings.TrimSpace(fs.Arg(0)))
	if provider != "xai" && provider != "xai-subscription" && provider != "grok" && provider != "supergrok" {
		return fmt.Errorf("unsupported login provider %q (supported: xai)", fs.Arg(0))
	}

	ctx, cancel := signalContext()
	defer cancel()
	device, err := xaisub.RequestDeviceCode(ctx, nil)
	if err != nil {
		return err
	}
	verifyURL := device.VerificationURIComplete
	if strings.TrimSpace(verifyURL) == "" {
		verifyURL = device.VerificationURI
	}
	fmt.Println("Sign in with SuperGrok or X Premium+")
	fmt.Printf("Open %s\n", verifyURL)
	if device.UserCode != "" {
		fmt.Printf("Enter code: %s\n", device.UserCode)
	}
	if !*noOpen {
		_ = openBrowser(verifyURL)
	}
	tokens, err := xaisub.WaitForDevice(ctx, device)
	if err != nil {
		return err
	}
	home := os.Getenv("HOME")
	if err := xaisub.PersistTokens(home, tokens, xaisub.DefaultBaseURL); err != nil {
		return err
	}
	if err := ensureXAISubscriptionProvider(home); err != nil {
		return err
	}
	fmt.Println("Signed in. Use provider xai-subscription, or set it as default_provider.")
	return nil
}

func ensureXAISubscriptionProvider(home string) error {
	configPath, err := statepath.ConfigPath(home)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	providers, _ := raw["providers"].(map[string]any)
	if providers == nil {
		providers = map[string]any{}
		raw["providers"] = providers
	}
	if _, exists := providers["xai-subscription"]; exists {
		return nil
	}
	providers["xai-subscription"] = map[string]any{
		"type":     "xai-subscription",
		"base_url": xaisub.DefaultBaseURL,
		"wire_api": "responses",
		"model":    xaisub.DefaultModel,
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return securefs.WriteFileAtomic(configPath, append(out, '\n'))
}

func openBrowser(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return errors.New("empty url")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}

func signalContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Minute)
}
