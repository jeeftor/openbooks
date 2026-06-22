package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/jeeftor/openbooks/staging"
)

// SeriesRegistry tracks series names seen during book saves for autocomplete suggestions.
type SeriesRegistry struct {
	mu       sync.RWMutex
	names    map[string]struct{}
	filePath string
}

// newSeriesRegistry creates the registry and loads any existing names from disk.
func newSeriesRegistry(downloadDir string) *SeriesRegistry {
	staging.EnsureStagingDir(downloadDir) //nolint:errcheck
	r := &SeriesRegistry{
		names:    make(map[string]struct{}),
		filePath: filepath.Join(staging.StagingDir(downloadDir), "series_names.json"),
	}
	r.load()
	return r
}

// AddIfNew records a series name if not already known and persists.
func (r *SeriesRegistry) AddIfNew(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.names[name]; ok {
		return
	}
	r.names[name] = struct{}{}
	r.persist() //nolint:errcheck
}

// All returns a sorted slice of all known series names.
func (r *SeriesRegistry) All() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.names))
	for name := range r.names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (r *SeriesRegistry) load() {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return
	}
	for _, name := range list {
		if name != "" {
			r.names[name] = struct{}{}
		}
	}
}

func (r *SeriesRegistry) persist() error {
	// Must NOT call r.All() here — caller holds write lock; RLock would deadlock.
	list := make([]string, 0, len(r.names))
	for name := range r.names {
		list = append(list, name)
	}
	sort.Strings(list)

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, r.filePath)
}
