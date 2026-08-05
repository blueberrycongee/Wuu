package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	wuuexec "github.com/blueberrycongee/wuu/internal/exec"
	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/statepath"
)

type pluginPackageOutput struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name,omitempty"`
	Version              string   `json:"version,omitempty"`
	Description          string   `json:"description,omitempty"`
	SourcePath           string   `json:"source_path,omitempty"`
	SourceKind           string   `json:"source_kind,omitempty"`
	Destination          string   `json:"destination,omitempty"`
	ManifestPath         string   `json:"manifest_path,omitempty"`
	FileCount            int      `json:"file_count,omitempty"`
	UnpackedSize         int64    `json:"unpacked_size,omitempty"`
	Fingerprint          string   `json:"fingerprint,omitempty"`
	RequestedPermissions []string `json:"requested_permissions,omitempty"`
	EffectivePermissions []string `json:"effective_permissions,omitempty"`
	UnsupportedFields    []string `json:"unsupported_fields,omitempty"`
	Source               string   `json:"source,omitempty"`
	Root                 string   `json:"root,omitempty"`
	SubjectID            string   `json:"subject_id,omitempty"`
	Replaced             bool     `json:"replaced,omitempty"`
	ApprovalRequired     bool     `json:"approval_required,omitempty"`
	Removed              bool     `json:"removed,omitempty"`
}

func runPlugin(args []string) error {
	if len(args) == 0 {
		return pluginCLIError(errors.New("plugin subcommand is required (available: inspect, install, list, remove)"))
	}
	switch args[0] {
	case "inspect":
		return runPluginInspect(args[1:])
	case "install":
		return runPluginInstall(args[1:])
	case "list":
		return runPluginList(args[1:])
	case "remove", "uninstall":
		return runPluginRemove(args[1:])
	default:
		return pluginCLIError(fmt.Errorf("unknown plugin subcommand %q (available: inspect, install, list, remove)", args[0]))
	}
}

func runPluginInspect(args []string) error {
	fs, jsonOutput := pluginFlagSet("plugin inspect")
	if err := fs.Parse(args); err != nil {
		return pluginCLIError(err)
	}
	source, err := requiredPluginArgument(fs, "plugin inspect requires one local directory or .zip path")
	if err != nil {
		return err
	}
	inspection, err := pluginpkg.InspectPackage(source)
	if err != nil {
		return pluginCLIError(err)
	}
	return printPluginPackageOutput(packageInspectionOutput(inspection), *jsonOutput, "Valid plugin package")
}

func runPluginInstall(args []string) error {
	fs, jsonOutput := pluginFlagSet("plugin install")
	if err := fs.Parse(args); err != nil {
		return pluginCLIError(err)
	}
	source, err := requiredPluginArgument(fs, "plugin install requires one local directory or .zip path")
	if err != nil {
		return err
	}
	home, err := statepath.Home("")
	if err != nil {
		return fmt.Errorf("resolve Wuu home: %w", err)
	}
	result, err := pluginpkg.InstallPackage(home, source)
	if err != nil {
		return pluginCLIError(err)
	}
	output := packageInspectionOutput(result.Package)
	output.Destination = result.Destination
	output.Replaced = result.Replaced
	output.ApprovalRequired = true
	return printPluginPackageOutput(output, *jsonOutput, "Installed plugin package; approval is required before code activation")
}

func runPluginList(args []string) error {
	fs, jsonOutput := pluginFlagSet("plugin list")
	workdir := fs.String("workdir", "", "workspace directory whose project plugins should be included")
	if err := fs.Parse(args); err != nil {
		return pluginCLIError(err)
	}
	if fs.NArg() != 0 {
		return pluginCLIError(errors.New("plugin list does not accept positional arguments"))
	}
	home, err := statepath.Home("")
	if err != nil {
		return fmt.Errorf("resolve Wuu home: %w", err)
	}
	root := strings.TrimSpace(*workdir)
	if root != "" {
		root, err = filepath.Abs(root)
		if err != nil {
			return pluginCLIError(fmt.Errorf("resolve workspace: %w", err))
		}
	}
	plugins := pluginpkg.Discover(root, home)
	output := make([]pluginPackageOutput, 0, len(plugins))
	for _, item := range plugins {
		output = append(output, pluginPackageOutput{
			ID:                   item.ID,
			Name:                 item.Name,
			Version:              item.Version,
			Description:          item.Description,
			Source:               item.Source,
			Root:                 item.Root,
			ManifestPath:         item.ManifestPath,
			Fingerprint:          item.Fingerprint,
			SubjectID:            item.SubjectID,
			RequestedPermissions: append([]string(nil), item.RequestedPermissions...),
			EffectivePermissions: append([]string(nil), item.EffectivePermissions...),
			UnsupportedFields:    append([]string(nil), item.UnsupportedFields...),
		})
	}
	if *jsonOutput {
		return printPluginJSON(output)
	}
	if len(output) == 0 {
		fmt.Println("No plugins found.")
		return nil
	}
	sort.Slice(output, func(i, j int) bool { return output[i].ID < output[j].ID })
	for _, item := range output {
		version := item.Version
		if version != "" {
			version = " " + version
		}
		fmt.Printf("%s%s\t%s\t%s\n", item.ID, version, item.Source, item.Root)
	}
	return nil
}

func runPluginRemove(args []string) error {
	fs, jsonOutput := pluginFlagSet("plugin remove")
	if err := fs.Parse(args); err != nil {
		return pluginCLIError(err)
	}
	id, err := requiredPluginArgument(fs, "plugin remove requires one installed plugin id")
	if err != nil {
		return err
	}
	home, err := statepath.Home("")
	if err != nil {
		return fmt.Errorf("resolve Wuu home: %w", err)
	}
	result, err := pluginpkg.UninstallPackage(home, id)
	if err != nil {
		return pluginCLIError(err)
	}
	output := pluginPackageOutput{ID: result.ID, Destination: result.Destination, Removed: result.Removed}
	message := "Plugin package was not installed"
	if result.Removed {
		message = "Removed plugin package"
	}
	return printPluginPackageOutput(output, *jsonOutput, message)
}

func pluginFlagSet(name string) (*flag.FlagSet, *bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs, fs.Bool("json", false, "output JSON")
}

func requiredPluginArgument(fs *flag.FlagSet, message string) (string, error) {
	if fs.NArg() != 1 || strings.TrimSpace(fs.Arg(0)) == "" {
		return "", pluginCLIError(errors.New(message))
	}
	return fs.Arg(0), nil
}

func packageInspectionOutput(value pluginpkg.PackageInspection) pluginPackageOutput {
	return pluginPackageOutput{
		ID:                   value.ID,
		Name:                 value.Name,
		Version:              value.Version,
		Description:          value.Description,
		SourcePath:           value.SourcePath,
		SourceKind:           string(value.SourceKind),
		ManifestPath:         value.ManifestPath,
		FileCount:            value.FileCount,
		UnpackedSize:         value.UnpackedSize,
		Fingerprint:          value.Fingerprint,
		RequestedPermissions: append([]string(nil), value.RequestedPermissions...),
		EffectivePermissions: append([]string(nil), value.EffectivePermissions...),
		UnsupportedFields:    append([]string(nil), value.UnsupportedFields...),
	}
}

func printPluginPackageOutput(output pluginPackageOutput, asJSON bool, message string) error {
	if asJSON {
		return printPluginJSON(output)
	}
	fmt.Printf("%s: %s\n", message, output.ID)
	if output.Version != "" {
		fmt.Printf("Version: %s\n", output.Version)
	}
	if output.Destination != "" {
		fmt.Printf("Location: %s\n", output.Destination)
	} else if output.SourcePath != "" {
		fmt.Printf("Source: %s\n", output.SourcePath)
	}
	if output.Fingerprint != "" {
		fmt.Printf("Fingerprint: %s\n", output.Fingerprint)
	}
	return nil
}

func printPluginJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugin output: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func pluginCLIError(err error) error {
	return wuuexec.WithExitCode(wuuexec.ExitInvalidInput, err)
}
