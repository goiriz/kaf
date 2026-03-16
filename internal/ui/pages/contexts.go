package pages

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/goiriz/kaf/internal/config"
	"github.com/goiriz/kaf/internal/ui/common"
	"github.com/goiriz/kaf/internal/ui/components"
)

type ContextListItem struct {
	Name string
	Ctx  config.Context
}

func (i ContextListItem) Title() string       { return i.Name }
func (i ContextListItem) Description() string { return fmt.Sprintf("Brokers: %v", i.Ctx.Brokers) }
func (i ContextListItem) FilterValue() string { return i.Name }

type contextDelegate struct{}

func (d contextDelegate) Height() int                               { return 2 }
func (d contextDelegate) Spacing() int                              { return 1 }
func (d contextDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d contextDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(ContextListItem)
	if !ok {
		return
	}

	title := i.Title()
	desc := i.Description()
	if i.Ctx.WriteEnabled {
		title += components.GlobalTheme.ErrorStyle.Render(" [WRITE ENABLED]")
	} else {
		title += components.GlobalTheme.InfoStyle.Render(" [READ-ONLY]")
	}

	if index == m.Index() {
		fmt.Fprint(w, components.GlobalTheme.SelectedStyle.Render("> "+title)+"\n  "+components.GlobalTheme.DimStyle.Render(desc))
	} else {
		fmt.Fprint(w, "  "+title+"\n  "+components.GlobalTheme.DimStyle.Render(desc))
	}
}

type ContextsModel struct {
	List list.Model
}

func NewContextsModel(cfg *config.Config) ContextsModel {
	var items []list.Item
	for name, ctx := range cfg.Contexts {
		items = append(items, ContextListItem{Name: name, Ctx: ctx})
	}

	l := list.New(items, contextDelegate{}, 0, 0)
	l.Title = "Switch Cluster Context"
	l.SetShowStatusBar(false)
	l.Styles.Title = components.GlobalTheme.TitleStyle

	return ContextsModel{List: l}
}

func (m *ContextsModel) Init() tea.Cmd { return nil }

func (m *ContextsModel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.List.FilterState() == 0 {
			switch msg.String() {
			case "esc", "q":
				return func() tea.Msg { return common.BackMsg{} }
			case "enter":
				if i, ok := m.List.SelectedItem().(ContextListItem); ok {
					return func() tea.Msg { return common.SwitchContextMsg{ContextName: i.Name} }
				}
			}
		}
	}

	m.List, cmd = m.List.Update(msg)
	return cmd
}

func (m *ContextsModel) HandleSize(w, h int) {
	m.List.SetSize(w, h)
}

func (m *ContextsModel) Close() {}

func (m *ContextsModel) View() string {
	return "\n" + m.List.View()
}
