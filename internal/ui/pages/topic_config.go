package pages

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui/common"
	"github.com/goiriz/kaf/internal/ui/components"
)

type ConfigItem struct {
	Entry kafka.ConfigEntry
}

func (i ConfigItem) Title() string { return i.Entry.Name }
func (i ConfigItem) Description() string {
	s := "Default"
	if !i.Entry.IsDefault {
		s = "Override"
	}
	if i.Entry.ReadOnly {
		s += " (Read-only)"
	}
	return fmt.Sprintf("Value: %s [%s]", i.Entry.Value, s)
}
func (i ConfigItem) FilterValue() string { return i.Entry.Name }

type TopicConfigModel struct {
	Client   kafka.Cluster
	Ctx      context.Context
	Topic    string
	List     list.Model
	Input    textinput.Model
	Editing  bool
	Selected string
	Loading  bool
}

func NewTopicConfigModel(ctx context.Context, client kafka.Cluster, topic string) TopicConfigModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Config: " + topic
	l.SetShowStatusBar(false)
	l.Styles.Title = components.TitleStyle
	ti := textinput.New()
	ti.Placeholder = "New value"
	return TopicConfigModel{Client: client, Ctx: ctx, Topic: topic, List: l, Input: ti, Loading: true}
}

func (m *TopicConfigModel) Init() tea.Cmd { return m.Fetch() }

func (m *TopicConfigModel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	if m.Editing {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "enter":
				v := m.Input.Value()
				m.Editing = false
				return m.UpdateConfig(m.Selected, v)
			case "esc":
				m.Editing = false
				return nil
			}
		}
		m.Input, cmd = m.Input.Update(msg)
		return cmd
	}

	switch msg := msg.(type) {
	case common.TopicConfigLoadedMsg:
		m.Loading = false
		items := make([]list.Item, len(msg.Entries))
		for i, e := range msg.Entries {
			items[i] = ConfigItem{Entry: e}
		}
		m.List.SetItems(items)
	case tea.KeyMsg:
		if m.List.FilterState() == list.Filtering {
			break
		}
		if msg.String() == "enter" {
			if !m.Client.IsWriteEnabled() {
				return func() tea.Msg { return common.ErrMsg{Err: kafka.ErrReadOnly} }
			}
			if i, ok := m.List.SelectedItem().(ConfigItem); ok && !i.Entry.ReadOnly {
				m.Editing = true
				m.Selected = i.Entry.Name
				m.Input.SetValue(i.Entry.Value)
				m.Input.Focus()
				return nil
			}
		}
	}
	m.List, cmd = m.List.Update(msg)
	return cmd
}

func (m *TopicConfigModel) HandleSize(w, h int) {
	m.List.SetSize(w, h)
}


func (m *TopicConfigModel) Close() {}

func (m *TopicConfigModel) View() string {
	if m.Loading {
		return "Loading configs..."
	}
	if m.Editing {
		return fmt.Sprintf("%s\n\n%s\n\n%s",
			components.GlobalTheme.SecondaryStyle.Render("EDIT: "+m.Selected),
			m.Input.View(), components.HelpStyle.Render("(enter: save, esc: cancel)"))
	}
	return m.List.View() + "\n" + components.HelpStyle.Render("enter: edit • esc: back")
}

func (m *TopicConfigModel) Fetch() tea.Cmd {
	return func() tea.Msg {
		entries, err := m.Client.GetTopicConfig(m.Ctx, m.Topic)
		if err != nil {
			return common.ErrMsg{Err: err}
		}
		return common.TopicConfigLoadedMsg{Entries: entries}
	}
}

func (m *TopicConfigModel) UpdateConfig(k, v string) tea.Cmd {
	return func() tea.Msg {
		if err := m.Client.UpdateTopicConfig(m.Ctx, m.Topic, k, v); err != nil {
			return common.ErrMsg{Err: err}
		}
		return m.Fetch()()
	}
}


