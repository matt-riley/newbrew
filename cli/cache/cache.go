// Package cache provides a file-based JSON cache for Homebrew formula data,
// stored under the user's cache directory (~/.cache/newbrew/formulae.json).
// The cache is considered fresh for 10 minutes.
package cache

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/matt-riley/newbrew/models"
)

const cacheExpiry = 10 * time.Minute

// Cache holds the timestamped formula data persisted to disk.
// Use NewCache to load (or initialise) a cache instance, then
// Save to persist and IsFresh to check staleness.
type Cache struct {
	Timestamp time.Time            // when the cache was last saved
	Formulae  []models.FormulaInfo // cached formula metadata
}

var cachePathFunc = defaultCachePath

func defaultCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	path := filepath.Join(dir, "newbrew", "formulae.json")
	return path
}

func cachePath() string {
	return cachePathFunc()
}

func ensureCacheDir() error {
	dir := filepath.Dir(cachePath())
	return os.MkdirAll(dir, 0o700)
}

// NewCache loads the on-disk cache (if it exists) and returns a *Cache
// ready for use. A missing cache file is not an error — the returned
// Cache will have zero Timestamp and empty Formulae.
func NewCache() (*Cache, error) {
	c := &Cache{}
	err := c.Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c, nil
		}
		return nil, err
	}
	return c, nil
}

// Load reads the JSON cache from disk into c. An io.EOF on an empty file
// is treated as a successful (no-op) load.
func (c *Cache) Load() error {
	path := cachePath()
	// #nosec G304 — cache path is deterministic (UserCacheDir/TempDir + "newbrew/formulae.json")
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	err = json.NewDecoder(f).Decode(c)
	if err == io.EOF {
		return nil
	}
	return err
}

// Save persists the given formulae to the cache file with the current
// timestamp. It creates the cache directory if needed.
func (c *Cache) Save(formulae []models.FormulaInfo) error {
	if c == nil {
		return errors.New("cache is nil")
	}
	if err := ensureCacheDir(); err != nil {
		return err
	}
	path := cachePath()
	// #nosec G304 — cache path is deterministic (UserCacheDir/TempDir + "newbrew/formulae.json")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	c.Timestamp = time.Now()
	c.Formulae = formulae

	if err := json.NewEncoder(f).Encode(c); err != nil {
		return err
	}
	return nil
}

// IsFresh reports whether the cache is still within the expiry window
// (cacheExpiry, currently 10 minutes) relative to its last save time.
func (c *Cache) IsFresh() bool {
	return time.Since(c.Timestamp) < cacheExpiry
}
