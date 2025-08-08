// main_test.go
package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCursorMovement(t *testing.T) {
	formulae := []FormulaInfo{
		{PRTitle: "foo 1.0.0 (new formula)", Desc: "Foo desc", Homepage: "https://foo.example.com"},
		{PRTitle: "bar 2.0.0 (new formula)", Desc: "Bar desc", Homepage: "https://bar.example.com"},
		{PRTitle: "baz 3.0.0 (new formula)", Desc: "Baz desc", Homepage: "https://baz.example.com"},
	}
	m := model{formulae: formulae, loaded: true}

	// Initial cursor should be 0
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}

	// Simulate down arrow
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = m2.(model)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after down, got %d", m.cursor)
	}

	// Simulate 'j'
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = m3.(model)
	if m.cursor != 2 {
		t.Errorf("expected cursor 2 after j, got %d", m.cursor)
	}

	// Simulate up arrow
	m4, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = m4.(model)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after up, got %d", m.cursor)
	}

	// Simulate 'k'
	m5, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = m5.(model)
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after k, got %d", m.cursor)
	}

	// Simulate up at top (should stay at 0)
	m6, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = m6.(model)
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after up at top, got %d", m.cursor)
	}

	// Simulate down at bottom (should stay at last)
	m.cursor = len(m.formulae) - 1
	m7, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = m7.(model)
	if m.cursor != len(m.formulae)-1 {
		t.Errorf("expected cursor at bottom, got %d", m.cursor)
	}
}

func TestOpenBrowserNotCalledOnInvalidHomepage(t *testing.T) {
	// We'll override openBrowser to test if it would be called
	called := false
	openBrowser = func(url string) error {
		called = true
		return nil
	}
	defer func() { openBrowser = realOpenBrowser }() // restore after test

	formulae := []FormulaInfo{
		{PRTitle: "foo", Desc: "desc", Homepage: "(not found)"},
	}
	m := model{formulae: formulae, loaded: true}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(model)
	if called {
		t.Errorf("openBrowser should not be called for '(not found)' homepage")
	}
}

func TestOpenBrowserCalled(t *testing.T) {
	called := false
	var gotURL string
	openBrowser = func(url string) error {
		called = true
		gotURL = url
		return nil
	}
	defer func() { openBrowser = realOpenBrowser }() // restore after test

	formulae := []FormulaInfo{
		{PRTitle: "foo", Desc: "desc", Homepage: "https://foo.example.com"},
	}
	m := model{formulae: formulae, loaded: true}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(model)
	if !called {
		t.Errorf("openBrowser should be called for valid homepage")
	}
	if gotURL != "https://foo.example.com" {
		t.Errorf("openBrowser called with wrong URL: %s", gotURL)
	}
}

// Save the real openBrowser so we can restore it after tests
var realOpenBrowser = openBrowser
