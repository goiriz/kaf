package pages

import (
	"context"
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui/common"
	"github.com/goiriz/kaf/internal/ui/components"
)

type GroupListItem struct {
	Group    kafkago.ListGroupsResponseGroup
	TotalLag int64
}

func (i GroupListItem) Title() string       { return i.Group.GroupID }
func (i GroupListItem) Description() string { return fmt.Sprintf("Protocol: %s | Lag: %d", i.Group.ProtocolType, i.TotalLag) }
func (i GroupListItem) FilterValue() string { return i.Group.GroupID }

type groupDelegate struct{ focused bool }

func (d groupDelegate) Height() int                               { return 1 }
func (d groupDelegate) Spacing() int                              { return 0 }
func (d groupDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d groupDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(GroupListItem)
	if !ok {
		return
	}

	lagStr := fmt.Sprintf(" [Lag: %d]", i.TotalLag)
	if i.TotalLag > 1000 {
		lagStr = components.GlobalTheme.ErrorStyle.Render(lagStr)
	} else if i.TotalLag > 0 {
		lagStr = components.GlobalTheme.SecondaryStyle.Render(lagStr)
	} else {
		lagStr = components.GlobalTheme.DimStyle.Render(lagStr)
	}

	str := fmt.Sprintf("%-40s %s", i.Group.GroupID, lagStr)

	style := lipgloss.NewStyle().PaddingLeft(2)
	if index == m.Index() {
		if d.focused {
			style = components.SelectedStyle.Copy().PaddingLeft(1)
			str = "> " + str
		} else {
			style = lipgloss.NewStyle().PaddingLeft(2).Foreground(components.SecondaryColor)
		}
	}
	fmt.Fprint(w, style.Render(str))
}

type GroupsModel struct {
	List         list.Model
	Client       kafka.Cluster
	Ctx          context.Context
	LastSelected string
	Loading      bool
	Focused      bool
}

func NewGroupsModel(ctx context.Context, client kafka.Cluster) GroupsModel {
	l := list.New([]list.Item{}, groupDelegate{focused: true}, 0, 0)
	l.Title = "Consumer Groups"
	l.SetShowStatusBar(false)
	l.Styles.Title = components.TitleStyle
	return GroupsModel{List: l, Client: client, Ctx: ctx, Loading: true, Focused: true}
}

func (m *GroupsModel) Init() tea.Cmd { return m.Fetch() }

func (m *GroupsModel) Update(msg tea.Msg) (tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case common.GroupsLoadedMsg:
		m.Loading = false
		items := make([]list.Item, len(msg.Groups))
		for i, g := range msg.Groups {
			items[i] = GroupListItem{Group: g.Group, TotalLag: g.TotalLag}
		}
		m.List.SetItems(items)
		m.syncSelection(&cmds)
	}

	var cmd tea.Cmd
	m.List, cmd = m.List.Update(msg)
	cmds = append(cmds, cmd)
	m.syncSelection(&cmds)
	return tea.Batch(cmds...)
}

func (m *GroupsModel) syncSelection(cmds *[]tea.Cmd) {
	if m.Loading || len(m.List.Items()) == 0 {
		return
	}
	if i, ok := m.List.SelectedItem().(GroupListItem); ok {
		if i.Group.GroupID != m.LastSelected {
			m.LastSelected = i.Group.GroupID
			*cmds = append(*cmds, func() tea.Msg { return common.GroupChangedMsg{GroupID: m.LastSelected, Delayed: false} })
		}
	}
}

func (m *GroupsModel) HandleSize(w, h int) {
	m.List.SetSize(w, h)
}

func (m *GroupsModel) Close() {}

func (m *GroupsModel) Reset() {
	m.List.ResetFilter()
}

func (m *GroupsModel) View() string {
	if m.Loading {
		return "Loading groups..."
	}

	if m.Focused {
		m.List.Styles.Title = components.TitleStyle
	} else {
		m.List.Styles.Title = components.InactiveTitleStyle
	}
	m.List.SetDelegate(groupDelegate{focused: m.Focused})

	view := m.List.View()
	return view
}

func (m *GroupsModel) Fetch() tea.Cmd {
	return func() tea.Msg {
		groups, err := m.Client.ListGroups(m.Ctx)
		if err != nil {
			return common.ErrMsg{Err: err}
		}
		
		res := make([]common.GroupWithLag, len(groups))
		for i, g := range groups {
			res[i] = common.GroupWithLag{Group: g}
		}
		return common.GroupsLoadedMsg{Groups: res}
	}
}


func (m *GroupListItem) Close() {}
