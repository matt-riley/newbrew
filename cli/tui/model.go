package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/matt-riley/newbrew/cache"
	"github.com/matt-riley/newbrew/fetcher"
	"github.com/matt-riley/newbrew/models"
)

const (
	defaultDays  = 5
	defaultLimit = 50
)

var (
	newCache  = cache.NewCache
	fetchData = func(f *fetcher.Fetcher, c fetcher.CacheInterface) (fetcher.Result, error) {
		return f.FetchAndCache(c)
	}
)

type Config struct {
	Days          int
	Limit         int
	UseCache      bool
	ClampWarnings []string
	Fetcher       *fetcher.Fetcher
}

type model struct {
	config     Config
	list       list.Model
	spinner    spinner.Model
	loaded     bool
	err        error
	cached     bool
	refreshing bool
	status     string
}

type initialLoadMsg struct {
	formulae     []models.FormulaInfo
	err          error
	cached       bool
	needsRefresh bool
	warnings     []string
}

type loadedMsg struct {
	formulae []models.FormulaInfo
	err      error
	cached   bool
	warnings []string
}

type browserOpenErrMsg struct {
	err error
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
	return NewModel(Config{UseCache: true})
}

func NewModel(config Config) model {
	config = normalizeConfig(config)

	s := spinner.New()
	s.Spinner = spinner.Dot

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.KeyMap = list.DefaultKeyMap()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "open"),
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

	return model{
		config:  config,
		spinner: s,
		list:    l,
	}
}

func normalizeConfig(config Config) Config {
	if config.Days <= 0 {
		config.Days = defaultDays
	}
	if config.Limit <= 0 {
		config.Limit = defaultLimit
	}
	if config.Fetcher == nil {
		config.Fetcher = fetcher.New(fetcher.Config{
			Days:  config.Days,
			Limit: config.Limit,
		})
	}
	return config
}

func (m model) Init() tea.Cmd {
	return tea.Batch(loadInitialDataCmd(m.config), m.spinner.Tick)
}

func loadInitialDataCmd(config Config) tea.Cmd {
	config = normalizeConfig(config)

	return func() tea.Msg {
		if config.UseCache {
			c, err := newCache()
			if err == nil && c.IsFresh() {
				return initialLoadMsg{
					formulae:     c.Formulae,
					cached:       true,
					needsRefresh: true,
					warnings:     config.ClampWarnings,
				}
			}
		}

		return fetchCmd(config)()
	}
}

func fetchCmd(config Config) tea.Cmd {
	config = normalizeConfig(config)

	return func() tea.Msg {
		cacheStore, warnings := cacheForFetch(config)
		result, err := fetchData(config.Fetcher, cacheStore)
		// Prepend clamp warnings (from main) before fetch warnings.
		if len(config.ClampWarnings) > 0 {
			result.Warnings = append(config.ClampWarnings, result.Warnings...)
		}
		if len(warnings) > 0 {
			result.Warnings = append(warnings, result.Warnings...)
		}
		return loadedMsg{
			formulae: result.Formulae,
			err:      err,
			cached:   false,
			warnings: result.Warnings,
		}
	}
}

func cacheForFetch(config Config) (fetcher.CacheInterface, []string) {
	if !config.UseCache {
		return nil, nil
	}

	c, err := newCache()
	if err != nil {
		return nil, []string{err.Error()}
	}

	return c, nil
}

func joinWarnings(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}
	return strings.Join(warnings, " | ")
}
