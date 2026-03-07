package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/matt-riley/newbrew/cache"
	"github.com/matt-riley/newbrew/fetcher"
)

var openBrowser = func(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

func isBrowsableHomepage(homepage string) bool {
	return strings.HasPrefix(homepage, "https://") || strings.HasPrefix(homepage, "http://")
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case loadedMsg:
		items := make([]list.Item, len(msg.formulae))
		for i, f := range msg.formulae {
			items[i] = formulaItem(f)
		}
		m.list.SetItems(items)
		m.loaded = true
		m.err = msg.err
		m.cached = msg.cached
		m.refreshing = false
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			m.loaded = false
			m.refreshing = true
			return m, tea.Batch(func() tea.Msg {
				c, err := cache.NewCache()
				if err != nil {
					return loadedMsg{nil, err, false}
				}
				formulae, err := fetcher.FetchAndCache(c)
				return loadedMsg{formulae, err, false}
			}, m.spinner.Tick)
		case "enter":
			if selected, ok := m.list.SelectedItem().(formulaItem); ok {
				homepage := strings.TrimSpace(selected.Homepage)
				if isBrowsableHomepage(homepage) {
					_ = openBrowser(homepage)
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
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}
