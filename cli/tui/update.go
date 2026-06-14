package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/matt-riley/newbrew/models"
)

var openBrowser = func(url string) error {
	cmd, err := browserCommand(runtime.GOOS, url)
	if err != nil {
		return err
	}
	return cmd.Start()
}

func browserCommand(goos, url string) (*exec.Cmd, error) {
	switch goos {
	case "darwin":
		return exec.Command("open", url), nil
	case "linux":
		return exec.Command("xdg-open", url), nil
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url), nil
	default:
		return nil, fmt.Errorf("unsupported platform")
	}
}

func isBrowsableHomepage(homepage string) bool {
	return strings.HasPrefix(homepage, "https://") || strings.HasPrefix(homepage, "http://")
}

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
					m.err = openBrowser(homepage)
					return m, nil
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
