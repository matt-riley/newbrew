package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/matt-riley/newbrew/cache"
	"github.com/matt-riley/newbrew/fetcher"
	"github.com/matt-riley/newbrew/models"
)

func loadedModelWithFormulae(formulae []models.FormulaInfo) model {
	m := InitialModel()
	items := make([]list.Item, len(formulae))
	for i, f := range formulae {
		items[i] = formulaItem(f)
	}
	m.list.SetItems(items)
	m.loaded = true
	return m
}

func TestCursorMovement(t *testing.T) {
	formulae := []models.FormulaInfo{
		{PRTitle: "foo 1.0.0 (new formula)", Desc: "Foo desc", Homepage: "https://foo.example.com"},
		{PRTitle: "bar 2.0.0 (new formula)", Desc: "Bar desc", Homepage: "https://bar.example.com"},
		{PRTitle: "baz 3.0.0 (new formula)", Desc: "Baz desc", Homepage: "https://baz.example.com"},
	}
	m := loadedModelWithFormulae(formulae)

	if m.list.Index() != 0 {
		t.Errorf("expected initial index 0, got %d", m.list.Index())
	}

	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = m2.(model)
	if m.list.Index() != 1 {
		t.Errorf("expected index 1 after down, got %d", m.list.Index())
	}

	m3, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = m3.(model)
	if m.list.Index() != 2 {
		t.Errorf("expected index 2 after j, got %d", m.list.Index())
	}

	m4, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = m4.(model)
	if m.list.Index() != 1 {
		t.Errorf("expected index 1 after up, got %d", m.list.Index())
	}

	m5, _ := m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m = m5.(model)
	if m.list.Index() != 0 {
		t.Errorf("expected index 0 after k, got %d", m.list.Index())
	}

	m6, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = m6.(model)
	if m.list.Index() != 0 {
		t.Errorf("expected index 0 at top boundary, got %d", m.list.Index())
	}

	m.list.Select(len(formulae) - 1)
	m7, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = m7.(model)
	if m.list.Index() != len(formulae)-1 {
		t.Errorf("expected index %d at bottom boundary, got %d", len(formulae)-1, m.list.Index())
	}
}

func TestOpenBrowserNotCalledOnInvalidHomepage(t *testing.T) {
	called := false
	openBrowser = func(url string) error {
		called = true
		return nil
	}
	defer func() { openBrowser = realOpenBrowser }()

	m := loadedModelWithFormulae([]models.FormulaInfo{
		{PRTitle: "foo", Desc: "desc", Homepage: "(not found)"},
	})

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if called {
		t.Errorf("openBrowser should not be called for non-URL homepage")
	}
}

func TestOpenBrowserCalledForValidHomepage(t *testing.T) {
	called := false
	var gotURL string
	openBrowser = func(url string) error {
		called = true
		gotURL = url
		return nil
	}
	defer func() { openBrowser = realOpenBrowser }()

	m := loadedModelWithFormulae([]models.FormulaInfo{
		{PRTitle: "foo", Desc: "desc", Homepage: "https://foo.example.com"},
	})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected enter to return a browser-open command")
	}
	_ = cmd()

	if !called {
		t.Errorf("openBrowser should be called for valid homepage")
	}
	if gotURL != "https://foo.example.com" {
		t.Errorf("openBrowser called with wrong URL: %s", gotURL)
	}
}

func TestLoadInitialDataCmdReturnsCachedDataAndStartsRefresh(t *testing.T) {
	originalNewCache := newCache
	originalFetch := fetchData
	t.Cleanup(func() {
		newCache = originalNewCache
		fetchData = originalFetch
	})

	newCache = func() (*cache.Cache, error) {
		return &cache.Cache{
			Timestamp: time.Now(),
			Formulae: []models.FormulaInfo{
				{PRTitle: "cached", Desc: "cached desc", Homepage: "https://cached.example.com"},
			},
		}, nil
	}
	fetchData = func(_ *fetcher.Fetcher, _ fetcher.CacheInterface) (fetcher.Result, error) {
		return fetcher.Result{
			Formulae: []models.FormulaInfo{
				{PRTitle: "fresh", Desc: "fresh desc", Homepage: "https://fresh.example.com"},
			},
		}, nil
	}

	m := NewModel(Config{Days: 5, UseCache: true})
	msg := loadInitialDataCmd(m.config)()
	initial, ok := msg.(initialLoadMsg)
	if !ok {
		t.Fatalf("expected initialLoadMsg, got %T", msg)
	}
	if !initial.cached {
		t.Fatalf("expected cached initial load")
	}
	if len(initial.formulae) != 1 || initial.formulae[0].PRTitle != "cached" {
		t.Fatalf("expected cached formula to be returned first")
	}

	nextModel, cmd := m.Update(initial)
	m = nextModel.(model)
	if !m.loaded || !m.cached || !m.refreshing {
		t.Fatalf("expected cached state with background refresh in progress")
	}

	refreshMsg := cmd()
	loaded, ok := refreshMsg.(loadedMsg)
	if !ok {
		t.Fatalf("expected loadedMsg, got %T", refreshMsg)
	}
	if loaded.cached {
		t.Fatalf("expected refreshed data to be marked uncached")
	}
	if len(loaded.formulae) != 1 || loaded.formulae[0].PRTitle != "fresh" {
		t.Fatalf("expected refreshed formula to be returned")
	}
}

func TestManualRefreshTogglesLoadingState(t *testing.T) {
	originalFetch := fetchData
	t.Cleanup(func() {
		fetchData = originalFetch
	})

	fetchData = func(_ *fetcher.Fetcher, _ fetcher.CacheInterface) (fetcher.Result, error) {
		return fetcher.Result{
			Formulae: []models.FormulaInfo{
				{PRTitle: "fresh", Desc: "fresh desc", Homepage: "https://fresh.example.com"},
			},
		}, nil
	}

	m := loadedModelWithFormulae([]models.FormulaInfo{
		{PRTitle: "old", Desc: "old desc", Homepage: "https://old.example.com"},
	})
	m.config = Config{Days: 5, UseCache: true}

	nextModel, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m = nextModel.(model)
	if m.loaded {
		t.Fatalf("expected model to enter loading state")
	}
	if !m.refreshing {
		t.Fatalf("expected refresh to be in progress")
	}

	msg := cmd()
	loaded, ok := msg.(loadedMsg)
	if !ok {
		t.Fatalf("expected loadedMsg, got %T", msg)
	}

	nextModel, _ = m.Update(loaded)
	m = nextModel.(model)
	if !m.loaded || m.refreshing {
		t.Fatalf("expected loaded state after refresh completes")
	}
}

func TestBrowserOpenFailureSurfacesError(t *testing.T) {
	openBrowser = func(url string) error {
		return errors.New("launch failed")
	}
	defer func() { openBrowser = realOpenBrowser }()

	m := loadedModelWithFormulae([]models.FormulaInfo{
		{PRTitle: "foo", Desc: "desc", Homepage: "https://foo.example.com"},
	})
	m.config = Config{Days: 5}

	nextModel, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = nextModel.(model)
	msg := cmd()
	nextModel, _ = m.Update(msg)
	m = nextModel.(model)

	if m.err == nil {
		t.Fatalf("expected browser open failure to set model error")
	}
	view := m.View().Content
	if !strings.Contains(view, "launch failed") {
		t.Fatalf("expected view to include browser error, got %q", view)
	}
}

func TestEmptyStateUsesConfiguredDays(t *testing.T) {
	m := NewModel(Config{Days: 5})
	m.loaded = true

	view := m.View().Content
	if !strings.Contains(view, "last 5 days") {
		t.Fatalf("expected empty state to mention configured days, got %q", view)
	}
}

func TestBrowserCommandSupportsWindows(t *testing.T) {
	cmd, err := browserCommand("windows", "https://foo.example.com")
	if err != nil {
		t.Fatalf("expected windows browser command, got error: %v", err)
	}
	if got := cmd.Path; !strings.Contains(strings.ToLower(got), "rundll32") {
		t.Fatalf("expected rundll32 command path, got %q", got)
	}
}

func TestBrowserCommandIncludesUnsupportedPlatformInError(t *testing.T) {
	_, err := browserCommand("plan9", "https://foo.example.com")
	if err == nil {
		t.Fatalf("expected unsupported platform error")
	}
	if !strings.Contains(err.Error(), "plan9") {
		t.Fatalf("expected platform in error, got %q", err.Error())
	}
}

var realOpenBrowser = openBrowser
