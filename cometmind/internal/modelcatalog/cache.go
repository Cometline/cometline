package modelcatalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/paths"
)

type diskCacheEnvelope struct {
	Version   int                      `json:"version"`
	Providers map[string]providerEntry `json:"providers"`
}

func modelsDevCachePath() (string, error) {
	d, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "models-dev.json"), nil
}

func readDiskCache() (*Catalog, bool) {
	path, err := cachePathFn()
	if err != nil {
		return nil, false
	}
	info, err := os.Stat(path)
	if err != nil || nowFn().Sub(info.ModTime()) >= CacheTTL {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	cat, err := parseDiskCache(data, info.ModTime())
	if err != nil {
		return nil, false
	}
	if !catalogHasInputModalities(cat) {
		// Pre-vision caches only stored limit fields; force a fresh fetch.
		return nil, false
	}
	return cat, true
}

func writeDiskCache(cat *Catalog) error {
	path, err := cachePathFn()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload := diskCacheEnvelope{
		Version:   diskCacheVersion,
		Providers: cat.Providers,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func parseDiskCache(data []byte, fetchedAt time.Time) (*Catalog, error) {
	var envelope diskCacheEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Version > 0 {
		if envelope.Version != diskCacheVersion {
			return nil, fmt.Errorf("models.dev cache version %d", envelope.Version)
		}
		if envelope.Providers == nil {
			envelope.Providers = map[string]providerEntry{}
		}
		return &Catalog{Providers: envelope.Providers, FetchedAt: fetchedAt}, nil
	}
	// Legacy caches were a bare provider map (and often missing modalities).
	return parseCatalog(data, fetchedAt)
}
