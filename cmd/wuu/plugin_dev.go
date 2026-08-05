package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/statepath"
)

// runPluginDev dispatches to the plugin developer subcommands.
func runPluginDev(args []string) error {
	if len(args) == 0 {
		return pluginCLIError(errors.New("plugin dev subcommand is required (available: create, validate, build, pack, dev)"))
	}
	switch args[0] {
	case "create":
		return runPluginCreate(args[1:])
	case "validate":
		return runPluginValidate(args[1:])
	case "build":
		return runPluginBuild(args[1:])
	case "pack":
		return runPluginPack(args[1:])
	case "dev":
		return runPluginDevMode(args[1:])
	default:
		return pluginCLIError(fmt.Errorf("unknown plugin dev subcommand %q (available: create, validate, build, pack, dev)", args[0]))
	}
}

// runPluginCreate scaffolds a new plugin directory.
func runPluginCreate(args []string) error {
	fs := flag.NewFlagSet("plugin create", flag.ExitOnError)
	output := fs.String("output", "", "Output directory (defaults to ./<name>)")
	pluginType := fs.String("type", "agent", "Plugin type: agent, desktop, or full")
	if err := fs.Parse(args); err != nil {
		return pluginCLIError(err)
	}
	if fs.NArg() == 0 {
		return pluginCLIError(errors.New("plugin create requires a plugin name"))
	}
	name := fs.Arg(0)

	if err := validatePluginName(name); err != nil {
		return pluginCLIError(err)
	}

	dir := strings.TrimSpace(*output)
	if dir == "" {
		dir = name
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return pluginCLIError(fmt.Errorf("create directory: %w", err))
	}

	manifest := map[string]any{
		"schema_version": 1,
		"id":             name,
		"name":           name,
		"version":        "0.1.0",
		"description":    fmt.Sprintf("A Wuu plugin: %s", name),
	}

	switch strings.TrimSpace(*pluginType) {
	case "agent":
		manifest["runtime"] = map[string]any{
			"protocol": "wuu-plugin-v1",
			"command":  "node",
			"args":     []string{"dist/index.js"},
		}
	case "desktop":
		manifest["desktop"] = map[string]any{"entry": "dist/index.js"}
	case "full":
		manifest["runtime"] = map[string]any{
			"protocol": "wuu-plugin-v1",
			"command":  "node",
			"args":     []string{"dist/runtime.js"},
		}
		manifest["desktop"] = map[string]any{"entry": "dist/renderer.js"}
	}

	manifestPath := filepath.Join(dir, "plugin.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return pluginCLIError(fmt.Errorf("marshal manifest: %w", err))
	}
	if err := os.WriteFile(manifestPath, append(data, '\n'), 0o644); err != nil {
		return pluginCLIError(fmt.Errorf("write manifest: %w", err))
	}

	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return pluginCLIError(fmt.Errorf("create src: %w", err))
	}

	indexPath := filepath.Join(srcDir, "index.ts")
	if err := os.WriteFile(indexPath, []byte(pluginScaffoldIndex(strings.TrimSpace(*pluginType))), 0o644); err != nil {
		return pluginCLIError(fmt.Errorf("write index.ts: %w", err))
	}

	pkgPath := filepath.Join(dir, "package.json")
	if err := os.WriteFile(pkgPath, []byte(pluginScaffoldPackageJSON(name)), 0o644); err != nil {
		return pluginCLIError(fmt.Errorf("write package.json: %w", err))
	}

	tsconfigPath := filepath.Join(dir, "tsconfig.json")
	if err := os.WriteFile(tsconfigPath, []byte(pluginScaffoldTSConfig()), 0o644); err != nil {
		return pluginCLIError(fmt.Errorf("write tsconfig.json: %w", err))
	}

	fmt.Printf("Created plugin %q in %s\n", name, dir)
	fmt.Printf("  manifest: %s\n", manifestPath)
	fmt.Printf("  entry:    %s\n", indexPath)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  cd %s\n", dir)
	fmt.Printf("  npm install\n")
	fmt.Printf("  npm run build\n")
	fmt.Printf("  wuu plugin dev .\n")
	return nil
}

func runPluginValidate(args []string) error {
	fs, jsonOutput := pluginFlagSet("plugin validate")
	if err := fs.Parse(args); err != nil {
		return pluginCLIError(err)
	}
	if fs.NArg() == 0 {
		return pluginCLIError(errors.New("plugin validate requires a plugin directory path"))
	}
	source := fs.Arg(0)

	inspection, err := pluginpkg.InspectPackage(source)
	if err != nil {
		return pluginCLIError(fmt.Errorf("validation failed: %w", err))
	}

	output := packageInspectionOutput(inspection)
	if *jsonOutput {
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("✓ Plugin %s v%s is valid\n", inspection.ID, inspection.Version)
	fmt.Printf("  fingerprint: %s\n", inspection.Fingerprint)
	fmt.Printf("  files:       %d (%d bytes)\n", inspection.FileCount, inspection.UnpackedSize)
	if len(inspection.RequestedPermissions) > 0 {
		fmt.Printf("  permissions: %s\n", strings.Join(inspection.RequestedPermissions, ", "))
	}
	if len(inspection.UnsupportedFields) > 0 {
		fmt.Printf("  ⚠ unsupported fields: %s\n", strings.Join(inspection.UnsupportedFields, ", "))
	}
	return nil
}

func runPluginBuild(args []string) error {
	fs := flag.NewFlagSet("plugin build", flag.ExitOnError)
	output := fs.String("output", "dist", "Output directory for built artifacts")
	if err := fs.Parse(args); err != nil {
		return pluginCLIError(err)
	}
	if fs.NArg() == 0 {
		return pluginCLIError(errors.New("plugin build requires a plugin directory path"))
	}
	source := fs.Arg(0)

	if _, err := pluginpkg.InspectPackage(source); err != nil {
		return pluginCLIError(fmt.Errorf("build: %w", err))
	}

	outDir := filepath.Join(source, strings.TrimSpace(*output))
	fmt.Printf("Build target: %s\n", outDir)
	fmt.Printf("Run your bundler to populate %s, then use 'wuu plugin pack' to distribute.\n", outDir)
	return nil
}

func runPluginPack(args []string) error {
	fs := flag.NewFlagSet("plugin pack", flag.ExitOnError)
	output := fs.String("output", "", "Output .zip path (defaults to <plugin-id>-<version>.zip)")
	if err := fs.Parse(args); err != nil {
		return pluginCLIError(err)
	}
	if fs.NArg() == 0 {
		return pluginCLIError(errors.New("plugin pack requires a plugin directory path"))
	}
	source := fs.Arg(0)

	inspection, err := pluginpkg.InspectPackage(source)
	if err != nil {
		return pluginCLIError(fmt.Errorf("pack: %w", err))
	}

	outPath := strings.TrimSpace(*output)
	if outPath == "" {
		outPath = fmt.Sprintf("%s-%s.zip", inspection.ID, inspection.Version)
	}

	result, err := pluginpkg.InstallPackage(source, outPath)
	if err != nil {
		return pluginCLIError(fmt.Errorf("pack: %w", err))
	}

	fmt.Printf("Packed %s v%s → %s\n", result.Package.ID, result.Package.Version, outPath)
	fmt.Printf("  fingerprint: %s\n", result.Package.Fingerprint)
	return nil
}

// DevAuthorization records a one-time dev directory grant.
type DevAuthorization struct {
	PluginID    string    `json:"plugin_id"`
	Directory   string    `json:"directory"`
	Token       string    `json:"token"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

func runPluginDevMode(args []string) error {
	fs := flag.NewFlagSet("plugin dev", flag.ExitOnError)
	watch := fs.Bool("watch", true, "Watch for file changes and auto-reload")
	pollInterval := fs.Duration("poll", 2*time.Second, "Poll interval for file watching")
	if err := fs.Parse(args); err != nil {
		return pluginCLIError(err)
	}
	if fs.NArg() == 0 {
		return pluginCLIError(errors.New("plugin dev requires a plugin directory path"))
	}
	dir := fs.Arg(0)

	inspection, err := pluginpkg.InspectPackage(dir)
	if err != nil {
		return pluginCLIError(fmt.Errorf("dev: %w", err))
	}

	wuuHome, err := statepath.Home("")
	if err != nil {
		return pluginCLIError(fmt.Errorf("dev: %w", err))
	}

	devDir := filepath.Join(wuuHome, "dev", "plugins")
	if err := os.MkdirAll(devDir, 0o700); err != nil {
		return pluginCLIError(fmt.Errorf("create dev dir: %w", err))
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return pluginCLIError(fmt.Errorf("resolve path: %w", err))
	}

	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	auth := &DevAuthorization{
		PluginID:    inspection.ID,
		Directory:   abs,
		Token:       token,
		Fingerprint: inspection.Fingerprint,
		CreatedAt:   time.Now(),
	}

	authPath := filepath.Join(devDir, inspection.ID+".json")
	authData, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return pluginCLIError(fmt.Errorf("marshal auth: %w", err))
	}
	if err := os.WriteFile(authPath, append(authData, '\n'), 0o600); err != nil {
		return pluginCLIError(fmt.Errorf("write auth: %w", err))
	}

	fmt.Printf("Dev mode authorized for %s v%s\n", inspection.ID, inspection.Version)
	fmt.Printf("  directory:  %s\n", dir)
	fmt.Printf("  auth token: %s\n", token)
	if *watch {
		fmt.Printf("  watching:   yes (poll interval %s)\n", pollInterval.String())
		fmt.Printf("  Save source files to trigger automatic reload.\n")
	}

	if *watch {
		watchDevDir(dir, *pollInterval)
	}

	return nil
}

func watchDevDir(dir string, interval time.Duration) {
	var lastMod time.Time

	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(lastMod) {
			lastMod = info.ModTime()
		}
		return nil
	})

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fmt.Printf("Watching for changes... (Ctrl+C to stop)\n")
	for range ticker.C {
		changed := false
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") && d.IsDir() {
				return filepath.SkipDir
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if info.ModTime().After(lastMod) {
				changed = true
				if info.ModTime().After(lastMod) {
					lastMod = info.ModTime()
				}
			}
			return nil
		})
		if changed {
			now := time.Now()
			lastMod = now
			fmt.Printf("[%s] Change detected — reload triggered\n", now.Format("15:04:05"))
		}
	}
}

func validatePluginName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("plugin name must not be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("plugin name %q is too long (max 64 chars)", name)
	}
	for i, c := range name {
		if i == 0 && (c < 'a' || c > 'z') {
			return fmt.Errorf("plugin name %q must start with a lowercase letter", name)
		}
		if c != '-' && c != '_' && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return fmt.Errorf("plugin name %q contains invalid character '%c'", name, c)
		}
	}
	return nil
}

func pluginScaffoldIndex(pluginType string) string {
	switch pluginType {
	case "agent":
		return fmt.Sprintf(`// @wuu/plugin-sdk agent runtime entry point
// This plugin contributes agent runtime capabilities.

export interface PluginContext {
  pluginId: string;
  generation: string;
  wuuHome: string;
}

export async function activate(ctx: PluginContext): Promise<void> {
  console.log("[%%s] activated generation %%s", ctx.pluginId, ctx.generation);
  // Register tools, system prompt sections, context providers, etc.
}

export async function deactivate(): Promise<void> {
  // Cleanup resources.
}
`)
	case "desktop":
		return fmt.Sprintf(`// @wuu/plugin-sdk desktop renderer entry point
// This plugin contributes desktop UI customizations.

import type { PluginGenerationApi } from "@wuu/plugin-sdk";

export function activate(api: PluginGenerationApi): void {
  // Register views, slots, surfaces, themes, CSS snippets, etc.
  console.log("[%%s] desktop plugin activated", api.pluginId);
}
`)
	default:
		return fmt.Sprintf(`// @wuu/plugin-sdk entry point
// This plugin contributes both agent runtime and desktop UI capabilities.

export async function activateRuntime(ctx: { pluginId: string }): Promise<void> {
  console.log("[%%s] runtime activated", ctx.pluginId);
}

export function activateDesktop(api: any): void {
  console.log("[%%s] desktop activated", api.pluginId);
}
`)
	}
}

func pluginScaffoldPackageJSON(name string) string {
	data, _ := json.MarshalIndent(map[string]any{
		"name":        name,
		"version":     "0.1.0",
		"private":     true,
		"type":        "module",
		"description": fmt.Sprintf("A Wuu plugin: %s", name),
		"scripts": map[string]string{
			"build":     "tsc",
			"typecheck": "tsc --noEmit",
		},
		"devDependencies": map[string]string{
			"typescript":       "^5.0.0",
			"@wuu/plugin-sdk": "workspace:*",
		},
	}, "", "  ")
	return string(data) + "\n"
}

func pluginScaffoldTSConfig() string {
	data, _ := json.MarshalIndent(map[string]any{
		"compilerOptions": map[string]any{
			"target":           "ES2022",
			"module":           "ESNext",
			"moduleResolution": "bundler",
			"outDir":           "dist",
			"rootDir":          "src",
			"strict":           true,
			"declaration":      true,
			"skipLibCheck":     true,
		},
		"include": []string{"src"},
	}, "", "  ")
	return string(data) + "\n"
}
