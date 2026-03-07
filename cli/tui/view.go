package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	docStyle        = lipgloss.NewStyle().Margin(1, 2)
	titleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	cachedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	paginationStyle = list.DefaultStyles(true).PaginationStyle.PaddingLeft(4)
	helpStyle       = list.DefaultStyles(true).HelpStyle.PaddingLeft(4).PaddingBottom(1)
)

func (m model) View() tea.View {
	if !m.loaded {
		if m.refreshing {
			return tea.NewView(fmt.Sprintf("%s Refreshing...\n", m.spinner.View()))
		}
		return tea.NewView(fmt.Sprintf("%s Loading new Homebrew formulae (labelled 'new formula')...\n", m.spinner.View()))
	}
	if m.err != nil {
		return tea.NewView(fmt.Sprintf("Error: %v\n", m.err))
	}
	if len(m.list.Items()) == 0 {
		return tea.NewView("No new formula PRs found in the last 2 days.\n")
	}
	var b strings.Builder
	if m.cached {
		b.WriteString(cachedStyle.Render("Using cached data. Press 'r' to refresh.\n"))
	}

	b.WriteString(m.list.View())
	b.WriteString("\nPress Enter to open homepage in browser.\n")
	return tea.NewView(b.String())
}
