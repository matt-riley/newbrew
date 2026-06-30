package tui

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/matt-riley/newbrew/models"
)

var openBrowser = func(rawURL string) error {
	cmd, err := browserCommand(runtime.GOOS, rawURL)
	if err != nil {
		return err
	}
	return cmd.Start()
}

func browserCommand(goos, rawURL string) (*exec.Cmd, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing URL %q: %w", rawURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", parsed.Scheme)
	}
	sanitized := parsed.String()

	switch goos {
	case "darwin":
		return exec.Command("open", sanitized), nil // #nosec G204 — URL is parsed and sanitized above
	case "linux":
		return exec.Command("xdg-open", sanitized), nil // #nosec G204 — URL is parsed and sanitized above
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", sanitized), nil // #nosec G204 — URL is parsed and sanitized above
	default:
		return nil, fmt.Errorf("unsupported platform: %s", goos)
	}
}

func isBrowsableHomepage(homepage string) bool {
	u, err := url.Parse(homepage)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		if err := openBrowser(url); err != nil {
			return browserOpenErrMsg{err: err}
		}
		return nil
	}
}

// Update handles incoming Bubble Tea messages (key presses, data loads,
// window resizes, spinner ticks) and returns the new model state plus any
// commands to execute.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case initialLoadMsg:
		m.setItems(msg.formulae)
		m.loaded = true
		m.err = msg.err
		m.cached = msg.cached
		m.refreshing = msg.needsRefresh
		m.status = joinWarnings(msg.warnings)
		if msg.needsRefresh {
			return m, fetchCmd(m.config)
		}
		return m, nil
	case loadedMsg:
		m.setItems(msg.formulae)
		m.loaded = true
		m.err = msg.err
		m.cached = msg.cached
		m.refreshing = false
		m.status = joinWarnings(msg.warnings)
		return m, nil
	case browserOpenErrMsg:
		m.err = msg.err
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			m.loaded = false
			m.refreshing = true
			m.err = nil
			m.status = ""
			return m, fetchCmd(m.config)
		case "enter":
			if selected, ok := m.list.SelectedItem().(formulaItem); ok {
				homepage := strings.TrimSpace(selected.Homepage)
				if isBrowsableHomepage(homepage) {
					return m, openBrowserCmd(homepage)
				}
			}
		}
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
		return m, nil
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *model) setItems(formulae []models.FormulaInfo) {
	items := make([]list.Item, len(formulae))
	for i, f := range formulae {
		items[i] = formulaItem(f)
	}
	m.list.SetItems(items)
}
