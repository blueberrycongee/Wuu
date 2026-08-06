package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverAuthorizedDevGenerationRejectsUnreceiptedAndTamperedPackages(t *testing.T) {
	home := t.TempDir()
	source := t.TempDir()
	writeDevGenerationTestFile(t, filepath.Join(source, "plugin.json"), `{"schema_version":1,"id":"dev-discovery","version":"1.0.0","desktop":{"entry":"desktop.js"}}`)
	writeDevGenerationTestFile(t, filepath.Join(source, "desktop.js"), "export const version = 'one';")

	legacySpoof := filepath.Join(home, "dev", "runtime", "plugins", "dev-discovery")
	writeDevGenerationTestFile(t, filepath.Join(legacySpoof, "plugin.json"), `{"schema_version":1,"id":"dev-discovery","version":"9.0.0"}`)
	unreceipted := filepath.Join(home, "dev", "generations", "dev-discovery", "package")
	writeDevGenerationTestFile(t, filepath.Join(unreceipted, "plugin.json"), `{"schema_version":1,"id":"dev-discovery","version":"9.0.0"}`)
	if found := findDevGenerationTestPlugin(Discover("", home), "dev-discovery"); found != nil {
		t.Fatalf("unreceipted dev package was discovered: %+v", found)
	}

	authorization := writeDevGenerationTestAuthorization(t, home, "dev-discovery", source)
	published, err := PublishDevGeneration(home, source, source, authorization)
	if err != nil {
		t.Fatalf("publish authorized generation: %v", err)
	}
	found := findDevGenerationTestPlugin(Discover("", home), "dev-discovery")
	if found == nil || !found.AuthorizedDev || found.Fingerprint != published.Fingerprint || found.Source != "dev" {
		t.Fatalf("authorized dev generation was not discovered: %+v", found)
	}

	writeDevGenerationTestFile(t, filepath.Join(found.Root, "desktop.js"), "export const version = 'tampered';")
	if found := findDevGenerationTestPlugin(Discover("", home), "dev-discovery"); found != nil {
		t.Fatalf("tampered dev generation was discovered: %+v", found)
	}
}

func TestPublishDevGenerationFailurePreservesPreviousGeneration(t *testing.T) {
	home := t.TempDir()
	source := t.TempDir()
	writeDevGenerationTestFile(t, filepath.Join(source, "plugin.json"), `{"schema_version":1,"id":"dev-rollback","version":"1.0.0","desktop":{"entry":"desktop.js"}}`)
	writeDevGenerationTestFile(t, filepath.Join(source, "desktop.js"), "export const version = 'stable';")
	authorization := writeDevGenerationTestAuthorization(t, home, "dev-rollback", source)
	first, err := PublishDevGeneration(home, source, source, authorization)
	if err != nil {
		t.Fatal(err)
	}

	invalid := t.TempDir()
	writeDevGenerationTestFile(t, filepath.Join(invalid, "plugin.json"), `{"schema_version":1,"id":"wrong-id","version":"2.0.0"}`)
	if _, err := PublishDevGeneration(home, source, invalid, authorization); err == nil {
		t.Fatal("invalid replacement succeeded")
	}
	found := findDevGenerationTestPlugin(Discover("", home), "dev-rollback")
	if found == nil || found.Fingerprint != first.Fingerprint {
		t.Fatalf("failed publish did not preserve previous generation: %+v", found)
	}
}

func writeDevGenerationTestAuthorization(t *testing.T, home, pluginID, directory string) DevAuthorization {
	t.Helper()
	abs, err := filepath.Abs(directory)
	if err != nil {
		t.Fatal(err)
	}
	authorization := DevAuthorization{PluginID: pluginID, Directory: abs, Token: "test-secret", CreatedAt: time.Now().UTC()}
	data, err := json.Marshal(authorization)
	if err != nil {
		t.Fatal(err)
	}
	path, err := DevAuthorizationPath(home, pluginID)
	if err != nil {
		t.Fatal(err)
	}
	writeDevGenerationTestFile(t, path, string(data))
	return authorization
}

func findDevGenerationTestPlugin(plugins []Plugin, id string) *Plugin {
	for index := range plugins {
		if plugins[index].ID == id {
			return &plugins[index]
		}
	}
	return nil
}

func writeDevGenerationTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
