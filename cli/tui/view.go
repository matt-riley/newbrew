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
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
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

	var b strings.Builder
	if m.cached {
		if m.refreshing {
			b.WriteString(cachedStyle.Render("Using cached data while refreshing...\n"))
		} else {
			b.WriteString(cachedStyle.Render("Using cached data. Press 'r' to refresh.\n"))
		}
	}
	if m.status != "" {
		b.WriteString(cachedStyle.Render(m.status))
		b.WriteByte('\n')
	}
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteByte('\n')
	}
	if len(m.list.Items()) == 0 {
		_, _ = fmt.Fprintf(&b, "No new formula PRs found in the last %s.\n", dayLabel(m.config.Days))
		return tea.NewView(b.String())
	}

	b.WriteString(m.list.View())
	b.WriteString("\nPress Enter to open homepage in browser.\n")
	return tea.NewView(b.String())
}

func dayLabel(days int) string {
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}
