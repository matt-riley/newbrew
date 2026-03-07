package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/matt-riley/newbrew/cache"
	"github.com/matt-riley/newbrew/fetcher"
	"github.com/matt-riley/newbrew/models"
)

type model struct {
	list       list.Model
	spinner    spinner.Model
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

type formulaItem models.FormulaInfo

func (i formulaItem) Title() string {
	if i.PRTitle == "" {
		return "(NO TITLE)"
	}
	return i.PRTitle
}
func (i formulaItem) Description() string { return i.Desc }
func (i formulaItem) FilterValue() string { return i.PRTitle }

func InitialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.KeyMap = list.DefaultKeyMap()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("⏎", "open"),
			),
			key.NewBinding(
				key.WithKeys("r"),
				key.WithHelp("r", "refresh"),
			),
		}
	}
	l.Title = "New Homebrew Formulae"
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle
	return model{spinner: s, list: l}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(func() tea.Msg {
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
	}, m.spinner.Tick)
}
