// main.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/matt-riley/newbrew/cache"
	"github.com/matt-riley/newbrew/fetcher"
	"github.com/matt-riley/newbrew/models"
)

// Bubble Tea TUI

type model struct {
	formulae   []models.FormulaInfo
	cursor     int
	loaded     bool
	err        error
	cached     bool
	refreshing bool
}

type loadedMsg struct {
	formulae []models.FormulaInfo
	err      error
	cached   bool
}

func (m model) Init() tea.Cmd {
	return func() tea.Msg {
		// Try cache first
		c, err := cache.NewCache()
		if err == nil && c.IsFresh() {
			// Show cached data immediately, then refresh in background
			go func() {
				formulae, err := fetcher.FetchAndCache(c)
				tea.Println("") // force redraw
				tea.NewProgram(model{}).Send(loadedMsg{formulae, err, false})
			}()
			return loadedMsg{c.Formulae, nil, true}
		}
		// No cache or stale, fetch and cache
		formulae, err := fetcher.FetchAndCache(c)
		return loadedMsg{formulae, err, false}
	}
}

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

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.loaded = true
		m.err = msg.err
		m.formulae = msg.formulae
		m.cached = msg.cached
		m.refreshing = false
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.formulae)-1 {
				m.cursor++
			}
		case "r":
			m.loaded = false
			m.refreshing = true
			return m, func() tea.Msg {
				c, err := cache.NewCache()
				formulae, err := fetcher.FetchAndCache(c)
				return loadedMsg{formulae, err, false}
			}
		case "enter":
			if m.cursor < len(m.formulae) && m.formulae[m.cursor].Homepage != "" && m.formulae[m.cursor].Homepage != "(not found)" && m.formulae[m.cursor].Homepage != "(error fetching homepage)" {
				_ = openBrowser(m.formulae[m.cursor].Homepage)
			}
		}
	}
	return m, nil
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	normalStyle   = lipgloss.NewStyle()
	descStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	homeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
	cachedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)
)

func (m model) View() string {
	if !m.loaded {
		if m.refreshing {
			return "Refreshing...\n"
		}
		return "Loading new Homebrew formulae (labelled 'new formula')...\n"
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	if len(m.formulae) == 0 {
		return "No new formula PRs found in the last 2 days.\n"
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("New Homebrew Formulae (last %d days)\n\n", 5)))
	if m.cached {
		b.WriteString(cachedStyle.Render("(cached, press r to refresh)\n\n"))
	}
	for i, f := range m.formulae {
		cursor := " "
		style := normalStyle
		if i == m.cursor {
			cursor = ">"
			style = selectedStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, f.PRTitle)))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if m.cursor < len(m.formulae) {
		f := m.formulae[m.cursor]
		b.WriteString(fmt.Sprintf(
			"%s\n%s\nHomepage: %s\nMerged: %s\n",
			titleStyle.Render(f.PRTitle),
			descStyle.Render(f.Desc),
			homeStyle.Render(f.Homepage),
			f.MergedAt.Format(time.RFC822),
		))
		b.WriteString("\nPress Enter to open homepage in browser.\n")
	}
	b.WriteString("\n↑/↓ or j/k to move, r to refresh, q to quit.\n")
	return b.String()
}

func main() {
	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
