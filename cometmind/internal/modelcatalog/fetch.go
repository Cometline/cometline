package modelcatalog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func load() (*Catalog, error) {
	mu.Lock()
	defer mu.Unlock()
	if cached != nil && nowFn().Sub(cached.FetchedAt) < CacheTTL {
		return cached, nil
	}
	if cached != nil && !refreshFailedAt.IsZero() && nowFn().Sub(refreshFailedAt) < RefreshBackoff {
		return cached, nil
	}
	if cat, ok := readDiskCache(); ok {
		cached = cat
		refreshFailedAt = time.Time{}
		return cat, nil
	}
	cat, err := fetchRemote()
	if err != nil {
		refreshFailedAt = nowFn()
		if cached != nil {
			return cached, nil
		}
		return nil, err
	}
	refreshFailedAt = time.Time{}
	cached = cat
	_ = writeDiskCache(cat)
	return cat, nil
}

func fetchRemote() (*Catalog, error) {
	req, err := http.NewRequest(http.MethodGet, fetchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "cometmind/modelcatalog")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models.dev: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	return parseCatalog(data, nowFn())
}

func parseCatalog(data []byte, fetchedAt time.Time) (*Catalog, error) {
	var providers map[string]providerEntry
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, fmt.Errorf("parse models.dev: %w", err)
	}
	if providers == nil {
		providers = map[string]providerEntry{}
	}
	return &Catalog{Providers: providers, FetchedAt: fetchedAt}, nil
}

func catalogHasInputModalities(cat *Catalog) bool {
	if cat == nil {
		return false
	}
	for _, provider := range cat.Providers {
		for _, model := range provider.Models {
			if len(model.Modalities.Input) > 0 {
				return true
			}
		}
	}
	return false
}
