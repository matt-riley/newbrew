package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matt-riley/newbrew/models"
)

func withTestCachePath(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	original := cachePathFunc
	cachePathFunc = func() string {
		return filepath.Join(tempDir, "formulae.json")
	}
	t.Cleanup(func() {
		cachePathFunc = original
	})

	return cachePathFunc()
}

func TestNewCacheMissingFileReturnsEmptyCache(t *testing.T) {
	withTestCachePath(t)

	c, err := NewCache()
	if err != nil {
		t.Fatalf("NewCache returned error: %v", err)
	}
	if !c.Timestamp.IsZero() {
		t.Fatalf("expected zero timestamp, got %v", c.Timestamp)
	}
	if len(c.Formulae) != 0 {
		t.Fatalf("expected empty formulae, got %d items", len(c.Formulae))
	}
}

func TestCacheSaveAndLoadRoundTrip(t *testing.T) {
	path := withTestCachePath(t)

	original := &Cache{}
	formulae := []models.FormulaInfo{
		{PRTitle: "foo", Desc: "desc", Homepage: "https://example.com"},
	}
	if err := original.Save(formulae); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected cache file to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected cache file to be non-empty")
	}

	loaded, err := NewCache()
	if err != nil {
		t.Fatalf("NewCache returned error: %v", err)
	}
	if len(loaded.Formulae) != 1 {
		t.Fatalf("expected one formula, got %d", len(loaded.Formulae))
	}
	if loaded.Formulae[0].PRTitle != "foo" {
		t.Fatalf("expected PRTitle foo, got %q", loaded.Formulae[0].PRTitle)
	}
	if loaded.Timestamp.IsZero() {
		t.Fatalf("expected timestamp to be set")
	}
}

func TestLoadReturnsErrorForCorruptJSON(t *testing.T) {
	path := withTestCachePath(t)

	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	c := &Cache{}
	if err := c.Load(); err == nil {
		t.Fatalf("expected Load to return an error for corrupt JSON")
	}
}

func TestIsFreshUsesExpiryBoundary(t *testing.T) {
	fresh := &Cache{Timestamp: time.Now().Add(-cacheExpiry + time.Second)}
	if !fresh.IsFresh() {
		t.Fatalf("expected cache inside expiry window to be fresh")
	}

	stale := &Cache{Timestamp: time.Now().Add(-cacheExpiry - time.Second)}
	if stale.IsFresh() {
		t.Fatalf("expected cache outside expiry window to be stale")
	}
}
