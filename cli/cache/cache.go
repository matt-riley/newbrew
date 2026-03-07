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

type Cache struct {
	Timestamp time.Time
	Formulae  []models.FormulaInfo
}

func cachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	path := filepath.Join(dir, "newbrew", "formulae.json")
	return path
}

func ensureCacheDir() error {
	dir := filepath.Dir(cachePath())
	return os.MkdirAll(dir, 0o755)
}

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

func (c *Cache) Load() error {
	path := cachePath()
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

func (c *Cache) Save(formulae []models.FormulaInfo) error {
	if c == nil {
		return errors.New("cache is nil")
	}
	if err := ensureCacheDir(); err != nil {
		return err
	}
	path := cachePath()
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

func (c *Cache) IsFresh() bool {
	return time.Since(c.Timestamp) < cacheExpiry
}
