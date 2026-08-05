package appserver

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blueberrycongee/wuu/internal/extensions"
	pluginpkg "github.com/blueberrycongee/wuu/internal/plugin"
	"github.com/blueberrycongee/wuu/internal/statepath"
)

func TestPluginPackageInspectZipWithoutRuntime(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "inspect.zip")
	writePluginPackageZip(t, archive, map[string]string{
		"inspect-demo/plugin.json": `{"id":"inspect-demo","name":"Inspect Demo","version":"1.2.3"}`,
		"inspect-demo/README.md":   "hello",
	})

	out := &lockedBuffer{}
	srv := &Server{out: out}
	callPluginPackageRPC(t, srv, "inspect", MethodPluginPackageInspect, PluginPackageInspectParams{Path: archive})
	response := responseByID(t, parseOutput(t, out.String()), "inspect")
	if response["error"] != nil {
		t.Fatalf("inspect response = %+v", response)
	}
	result := remarshal[PluginPackageInspectResult](t, response["result"])
	if result.Package.ID != "inspect-demo" || result.Package.Name != "Inspect Demo" || result.Package.Version != "1.2.3" {
		t.Fatalf("package metadata = %+v", result.Package)
	}
	if result.Package.SourceKind != PluginPackageSourceZip || result.Package.ArchiveRoot != "inspect-demo" || result.Package.ManifestPath != "inspect-demo/plugin.json" {
		t.Fatalf("package source metadata = %+v", result.Package)
	}
	if result.Package.FileCount != 2 || result.Package.Fingerprint == "" {
		t.Fatalf("package inspection stats = %+v", result.Package)
	}
}

func TestPluginPackageInstallDirectoryIsPendingAndDoesNotActivate(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	rt.WuuHome = retryingTempDir(t)
	source := t.TempDir()
	marker := filepath.Join(source, "executed")
	writePluginPackageFile(t, filepath.Join(source, "plugin.json"), `{
  "id": "pending-demo",
  "name": "Pending Demo",
  "version": "2.0.0",
  "skills": ["skills"],
  "runtime": {"protocol":"wuu-plugin-v1","command":"bin/plugin"}
}`)
	writePluginPackageFile(t, filepath.Join(source, "skills", "pending-skill", "SKILL.md"), "pending skill")
	writePluginPackageFile(t, filepath.Join(source, "bin", "plugin"), "#!/bin/sh\ntouch "+marker+"\n")

	out := &lockedBuffer{}
	srv := New(rt, out)
	callPluginPackageRPC(t, srv, "install", MethodPluginPackageInstall, PluginPackageInstallParams{Path: source})
	response := responseByID(t, parseOutput(t, out.String()), "install")
	if response["error"] != nil {
		t.Fatalf("install response = %+v", response)
	}
	result := remarshal[PluginPackageInstallResult](t, response["result"])
	if result.Package.ID != "pending-demo" || result.Package.SourceKind != PluginPackageSourceDirectory || result.Replaced {
		t.Fatalf("install result = %+v", result)
	}
	record := pluginPackageRecord(t, result.ExtensionInventory, "pending-demo")
	if record.ApprovalState != ExtensionApprovalPending || record.State != ExtensionStatePending || record.RuntimeState != ExtensionRuntimeInactive {
		t.Fatalf("pending inventory record = %+v", record)
	}
	if len(rt.ActivePlugins) != 0 {
		t.Fatalf("active plugins = %+v, want no unapproved plugin", rt.ActivePlugins)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("plugin runtime executed during install: %v", err)
	}
	installedManifest := filepath.Join(rt.WuuHome, "plugins", "pending-demo", "plugin.json")
	if _, err := os.Stat(installedManifest); err != nil {
		t.Fatalf("installed manifest under Wuu home: %v", err)
	}
	for _, skill := range result.Skills {
		if skill.Name == "pending-skill" {
			t.Fatalf("unapproved plugin skill was activated: %+v", skill)
		}
	}
}

func TestPluginPackageUpdateWaitsForExactApprovalBeforePromotion(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	rt.WuuHome = retryingTempDir(t)
	configPath, err := statepath.ConfigPath(rt.HomeDir)
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}
	writePluginPackageFile(t, configPath, `{
  "default_provider": "fake-provider",
  "providers": {"fake-provider": {"type": "openai-compatible", "base_url": "https://example.test/v1", "model": "fake-model"}}
}`)

	versionOne := writeManagedPluginPackage(t, "update-demo", "1.0.0", "first generation")
	out := &lockedBuffer{}
	srv := New(rt, out)
	callPluginPackageRPC(t, srv, "install-v1", MethodPluginPackageInstall, PluginPackageInstallParams{Path: versionOne})
	installResponse := responseByID(t, parseOutput(t, out.String()), "install-v1")
	installed := remarshal[PluginPackageInstallResult](t, installResponse["result"])
	installedRecord := pluginPackageRecord(t, installed.ExtensionInventory, "update-demo")
	callPluginPackageRPC(t, srv, "grant-v1", MethodExtensionPackageUpdate, ExtensionPackageUpdateParams{
		ID: installedRecord.ID, Fingerprint: installedRecord.Fingerprint, Action: ExtensionPackageGrant,
	})
	if response := responseByID(t, parseOutput(t, out.String()), "grant-v1"); response["error"] != nil {
		t.Fatalf("grant v1 response = %+v", response)
	}

	versionTwo := writeManagedPluginPackage(t, "update-demo", "2.0.0", "second generation")
	callPluginPackageRPC(t, srv, "stage-v2", MethodPluginPackageInstall, PluginPackageInstallParams{Path: versionTwo})
	stageResponse := responseByID(t, parseOutput(t, out.String()), "stage-v2")
	if stageResponse["error"] != nil {
		t.Fatalf("stage v2 response = %+v", stageResponse)
	}
	staged := remarshal[PluginPackageInstallResult](t, stageResponse["result"])
	if !staged.Pending || staged.Replaced || staged.ActiveFingerprint != installed.Package.Fingerprint {
		t.Fatalf("staged result = %+v", staged)
	}
	stagedRecord := pluginPackageRecord(t, staged.ExtensionInventory, "update-demo")
	if stagedRecord.PendingUpdate == nil || stagedRecord.PendingUpdate.Version != "2.0.0" || stagedRecord.PendingUpdate.Fingerprint != staged.Package.Fingerprint {
		t.Fatalf("staged inventory record = %+v", stagedRecord)
	}
	if len(rt.Plugins) != 1 || rt.Plugins[0].Version != "1.0.0" || len(rt.ActivePlugins) != 1 || rt.ActivePlugins[0].Version != "1.0.0" {
		t.Fatalf("active generation changed before approval: plugins=%+v active=%+v", rt.Plugins, rt.ActivePlugins)
	}

	callPluginPackageRPC(t, srv, "stale-promote", MethodExtensionPackageUpdate, ExtensionPackageUpdateParams{
		ID: stagedRecord.ID, Fingerprint: installed.Package.Fingerprint, Action: ExtensionPackagePromoteUpdate,
	})
	if response := responseByID(t, parseOutput(t, out.String()), "stale-promote"); response["error"] == nil {
		t.Fatalf("stale promotion response = %+v", response)
	}
	if len(rt.ActivePlugins) != 1 || rt.ActivePlugins[0].Version != "1.0.0" {
		t.Fatalf("stale approval changed active generation: %+v", rt.ActivePlugins)
	}

	callPluginPackageRPC(t, srv, "promote-v2", MethodExtensionPackageUpdate, ExtensionPackageUpdateParams{
		ID: stagedRecord.ID, Fingerprint: staged.Package.Fingerprint, Action: ExtensionPackagePromoteUpdate,
	})
	promoteResponse := responseByID(t, parseOutput(t, out.String()), "promote-v2")
	if promoteResponse["error"] != nil {
		t.Fatalf("promote v2 response = %+v", promoteResponse)
	}
	promoted := remarshal[ExtensionPackageUpdateResult](t, promoteResponse["result"])
	promotedRecord := pluginPackageRecord(t, promoted.ExtensionInventory, "update-demo")
	if promotedRecord.PendingUpdate != nil || promotedRecord.Fingerprint != staged.Package.Fingerprint {
		t.Fatalf("promoted inventory record = %+v", promotedRecord)
	}
	if len(rt.ActivePlugins) != 1 || rt.ActivePlugins[0].Version != "2.0.0" {
		t.Fatalf("approved generation was not activated: %+v", rt.ActivePlugins)
	}
	if rt.ExtensionSettings == nil {
		t.Fatal("approved update settings were not applied")
	}
	grant, ok := rt.ExtensionSettings.FindGrant(stagedRecord.ID, staged.Package.Fingerprint)
	if !ok || grant.Fingerprint != staged.Package.Fingerprint {
		t.Fatalf("approved update grant = %+v, %v", grant, ok)
	}

	versionThree := writeManagedPluginPackage(t, "update-demo", "3.0.0", "third generation")
	callPluginPackageRPC(t, srv, "stage-v3", MethodPluginPackageInstall, PluginPackageInstallParams{Path: versionThree})
	stageThreeResponse := responseByID(t, parseOutput(t, out.String()), "stage-v3")
	stageThree := remarshal[PluginPackageInstallResult](t, stageThreeResponse["result"])
	callPluginPackageRPC(t, srv, "reject-v3", MethodExtensionPackageUpdate, ExtensionPackageUpdateParams{
		ID: stagedRecord.ID, Fingerprint: stageThree.Package.Fingerprint, Action: ExtensionPackageRejectUpdate,
	})
	rejectResponse := responseByID(t, parseOutput(t, out.String()), "reject-v3")
	if rejectResponse["error"] != nil {
		t.Fatalf("reject v3 response = %+v", rejectResponse)
	}
	if len(rt.ActivePlugins) != 1 || rt.ActivePlugins[0].Version != "2.0.0" {
		t.Fatalf("rejection changed active generation: %+v", rt.ActivePlugins)
	}
	if _, err := pluginpkg.ReadPendingUpdate(rt.WuuHome, "update-demo"); !errors.Is(err, pluginpkg.ErrPendingUpdateNotFound) {
		t.Fatalf("pending update after rejection = %v", err)
	}
}

func TestPluginPackageRemoveRefreshesInventory(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	rt.WuuHome = retryingTempDir(t)
	configPath, err := statepath.ConfigPath(rt.HomeDir)
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}
	writePluginPackageFile(t, configPath, `{
  "default_provider": "fake-provider",
  "providers": {"fake-provider": {"type": "openai-compatible", "base_url": "https://example.test/v1", "model": "fake-model"}}
}`)
	source := t.TempDir()
	writePluginPackageFile(t, filepath.Join(source, "plugin.json"), `{"id":"remove-demo","skills":["skills"]}`)
	writePluginPackageFile(t, filepath.Join(source, "skills", "remove-skill", "SKILL.md"), `---
name: remove-skill
description: Verifies plugin package removal.
---

# Remove skill
`)

	out := &lockedBuffer{}
	srv := New(rt, out)
	callPluginPackageRPC(t, srv, "install", MethodPluginPackageInstall, PluginPackageInstallParams{Path: source})
	installResponse := responseByID(t, parseOutput(t, out.String()), "install")
	if installResponse["error"] != nil {
		t.Fatalf("install response = %+v", installResponse)
	}
	installed := remarshal[PluginPackageInstallResult](t, installResponse["result"])
	installedRecord := pluginPackageRecord(t, installed.ExtensionInventory, "remove-demo")
	callPluginPackageRPC(t, srv, "grant", MethodExtensionPackageUpdate, ExtensionPackageUpdateParams{
		ID: installedRecord.ID, Fingerprint: installedRecord.Fingerprint, Action: ExtensionPackageGrant,
	})
	grantResponse := responseByID(t, parseOutput(t, out.String()), "grant")
	if grantResponse["error"] != nil {
		t.Fatalf("grant response = %+v", grantResponse)
	}
	if len(rt.ActivePlugins) != 1 || rt.ActivePlugins[0].ID != "remove-demo" {
		t.Fatalf("active plugins before removal = %+v", rt.ActivePlugins)
	}
	foundSkill := false
	for _, skill := range rt.Skills {
		foundSkill = foundSkill || skill.Name == "remove-skill"
	}
	if !foundSkill {
		t.Fatalf("approved plugin skill was not activated: %+v", rt.Skills)
	}

	callPluginPackageRPC(t, srv, "remove", MethodPluginPackageRemove, PluginPackageRemoveParams{ID: "remove-demo"})
	response := responseByID(t, parseOutput(t, out.String()), "remove")
	if response["error"] != nil {
		t.Fatalf("remove response = %+v", response)
	}
	result := remarshal[PluginPackageRemoveResult](t, response["result"])
	if result.ID != "remove-demo" || !result.Removed {
		t.Fatalf("remove result = %+v", result)
	}
	for _, record := range result.ExtensionInventory {
		if record.Provenance.PluginID == "remove-demo" {
			t.Fatalf("removed plugin remains in inventory: %+v", record)
		}
	}
	if _, err := os.Stat(filepath.Join(rt.WuuHome, "plugins", "remove-demo")); !os.IsNotExist(err) {
		t.Fatalf("removed plugin still exists: %v", err)
	}
	if len(rt.ActivePlugins) != 0 {
		t.Fatalf("removed plugin remains active: %+v", rt.ActivePlugins)
	}
	for _, skill := range rt.Skills {
		if skill.Name == "remove-skill" {
			t.Fatalf("removed plugin skill remains active: %+v", skill)
		}
	}
	if rt.ExtensionSettings != nil {
		if _, ok := rt.ExtensionSettings.Grants[extensions.SubjectID("user", "remove-demo")]; ok {
			t.Fatalf("removed plugin grant remains persisted: %+v", rt.ExtensionSettings.Grants)
		}
	}
}

func TestPluginPackageHandlersRejectInvalidPathAndID(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	rt.WuuHome = retryingTempDir(t)
	out := &lockedBuffer{}
	srv := New(rt, out)

	callPluginPackageRPC(t, srv, "bad-path", MethodPluginPackageInstall, PluginPackageInstallParams{Path: filepath.Join(t.TempDir(), "missing")})
	callPluginPackageRPC(t, srv, "bad-id", MethodPluginPackageRemove, PluginPackageRemoveParams{ID: "../outside"})
	messages := parseOutput(t, out.String())
	pathError := responseErrorMessage(t, responseByID(t, messages, "bad-path"))
	if !strings.Contains(pathError, "install plugin package") || !strings.Contains(pathError, "missing") {
		t.Fatalf("invalid path error = %q", pathError)
	}
	idError := responseErrorMessage(t, responseByID(t, messages, "bad-id"))
	if !strings.Contains(idError, "remove plugin package") || !strings.Contains(idError, "portable path component") {
		t.Fatalf("invalid id error = %q", idError)
	}
	if _, err := os.Stat(filepath.Join(rt.WuuHome, "plugins")); !os.IsNotExist(err) {
		t.Fatalf("invalid requests mutated plugin directory: %v", err)
	}
}

func TestPluginPackageMutationsRejectRunningTurn(t *testing.T) {
	rt := newTestRuntime(t, &fakeClient{})
	rt.WuuHome = retryingTempDir(t)
	source := t.TempDir()
	writePluginPackageFile(t, filepath.Join(source, "plugin.json"), `{"id":"busy-demo"}`)
	out := &lockedBuffer{}
	srv := New(rt, out)
	thread := &threadState{ID: "busy", running: true}
	srv.mu.Lock()
	srv.threads[thread.ID] = thread
	srv.mu.Unlock()

	callPluginPackageRPC(t, srv, "install-busy", MethodPluginPackageInstall, PluginPackageInstallParams{Path: source})
	callPluginPackageRPC(t, srv, "remove-busy", MethodPluginPackageRemove, PluginPackageRemoveParams{ID: "busy-demo"})
	messages := parseOutput(t, out.String())
	for _, id := range []string{"install-busy", "remove-busy"} {
		message := responseErrorMessage(t, responseByID(t, messages, id))
		if !strings.Contains(message, "while a turn is running") {
			t.Fatalf("%s error = %q", id, message)
		}
	}
	if _, err := os.Stat(filepath.Join(rt.WuuHome, "plugins", "busy-demo")); !os.IsNotExist(err) {
		t.Fatalf("running-turn rejection mutated package: %v", err)
	}
}

func callPluginPackageRPC(t *testing.T, srv *Server, id, method string, params any) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.handleLine(context.Background(), raw); err != nil {
		t.Fatalf("call %s: %v", method, err)
	}
}

func pluginPackageRecord(t *testing.T, records []ExtensionInventoryRecord, pluginID string) ExtensionInventoryRecord {
	t.Helper()
	for _, record := range records {
		if record.Provenance.PluginID == pluginID && record.Kind == "plugin" {
			return record
		}
	}
	t.Fatalf("plugin %q not found in inventory: %+v", pluginID, records)
	return ExtensionInventoryRecord{}
}

func responseErrorMessage(t *testing.T, response map[string]any) string {
	t.Helper()
	if response["error"] == nil {
		t.Fatalf("response unexpectedly succeeded: %+v", response)
	}
	return remarshal[ResponseError](t, response["error"]).Message
}

func writePluginPackageFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeManagedPluginPackage(t *testing.T, id, version, contents string) string {
	t.Helper()
	source := t.TempDir()
	writePluginPackageFile(t, filepath.Join(source, "plugin.json"), fmt.Sprintf(`{"id":%q,"version":%q,"skills":["skills"]}`, id, version))
	writePluginPackageFile(t, filepath.Join(source, "skills", "managed-skill", "SKILL.md"), fmt.Sprintf(`---
name: managed-skill
description: Verifies managed plugin generations.
---

# Managed skill

%s
`, contents))
	return source
}

func writePluginPackageZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, contents := range entries {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
