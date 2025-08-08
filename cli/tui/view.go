package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

var (
	docStyle        = lipgloss.NewStyle().Margin(1, 2)
	titleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	cachedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	paginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle       = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
)

func (m model) View() string {
	if !m.loaded {
		if m.refreshing {
			return fmt.Sprintf("%s Refreshing...\n", m.spinner.View())
		}
		return fmt.Sprintf("%s Loading new Homebrew formulae (labelled 'new formula')...\n", m.spinner.View())
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	if len(m.list.Items()) == 0 {
		return "No new formula PRs found in the last 2 days.\n"
	}
	var b strings.Builder
	if m.cached {
		b.WriteString(cachedStyle.Render("Using cached data. Press 'r' to refresh.\n"))
	}

	b.WriteString(m.list.View())
	b.WriteString("\nPress Enter to open homepage in browser.\n")
	return b.String()
}
