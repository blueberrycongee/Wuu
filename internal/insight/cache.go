package insight

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const cacheSubDir = "insight-cache"

// CacheDir returns the cache directory: .wuu/insight-cache/
func CacheDir(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, ".wuu", cacheSubDir)
}

// LoadCachedMeta loads cached session metadata, returns nil if missing.
func LoadCachedMeta(cacheDir, sessionID string) *SessionMeta {
	path := filepath.Join(cacheDir, sessionID+".meta.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var meta SessionMeta
	if json.Unmarshal(data, &meta) != nil {
		return nil
	}
	return &meta
}

// SaveCachedMeta persists session metadata to the cache.
func SaveCachedMeta(cacheDir string, meta SessionMeta) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, meta.ID+".meta.json"), data, 0o600)
}

// LoadCachedFacet loads a cached facet, returns nil if missing.
func LoadCachedFacet(cacheDir, sessionID string) *Facet {
	path := filepath.Join(cacheDir, sessionID+".facet.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var facet Facet
	if json.Unmarshal(data, &facet) != nil {
		return nil
	}
	return &facet
}

// SaveCachedFacet persists a facet to the cache.
func SaveCachedFacet(cacheDir string, facet Facet) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(facet)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, facet.SessionID+".facet.json"), data, 0o600)
}
