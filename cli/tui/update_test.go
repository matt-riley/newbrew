package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

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

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = m2.(model)
	if m.list.Index() != 1 {
		t.Errorf("expected index 1 after down, got %d", m.list.Index())
	}

	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = m3.(model)
	if m.list.Index() != 2 {
		t.Errorf("expected index 2 after j, got %d", m.list.Index())
	}

	m4, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = m4.(model)
	if m.list.Index() != 1 {
		t.Errorf("expected index 1 after up, got %d", m.list.Index())
	}

	m5, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = m5.(model)
	if m.list.Index() != 0 {
		t.Errorf("expected index 0 after k, got %d", m.list.Index())
	}

	m6, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = m6.(model)
	if m.list.Index() != 0 {
		t.Errorf("expected index 0 at top boundary, got %d", m.list.Index())
	}

	m.list.Select(len(formulae) - 1)
	m7, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
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

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

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

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !called {
		t.Errorf("openBrowser should be called for valid homepage")
	}
	if gotURL != "https://foo.example.com" {
		t.Errorf("openBrowser called with wrong URL: %s", gotURL)
	}
}

var realOpenBrowser = openBrowser
