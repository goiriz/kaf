package pages

import (
	"context"
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui/common"
	"github.com/goiriz/kaf/internal/ui/components"
)

type TopicListItem struct {
	Topic    kafkago.Topic
	Deleting bool
}

func (i TopicListItem) Title() string       { return i.Topic.Name }
func (i TopicListItem) Description() string { return fmt.Sprintf("P:%d", len(i.Topic.Partitions)) }
func (i TopicListItem) FilterValue() string { return i.Topic.Name }

type topicDelegate struct{ focused bool }

func (d topicDelegate) Height() int                               { return 1 }
func (d topicDelegate) Spacing() int                              { return 0 }
func (d topicDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d topicDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(TopicListItem)
	if !ok {
		return
	}

	str := fmt.Sprintf("%-30s [P:%d]", i.Topic.Name, len(i.Topic.Partitions))
	if i.Deleting {
		str += " [DELETING...]"
	}

	style := lipgloss.NewStyle().PaddingLeft(2)
	if i.Deleting {
		style = components.GlobalTheme.DeletingStyle
	} else if index == m.Index() {
		if d.focused {
			style = components.GlobalTheme.SelectedStyle.Copy().PaddingLeft(1)
			str = "> " + str
		} else {
			style = lipgloss.NewStyle().PaddingLeft(2).Foreground(components.GlobalTheme.SecondaryColor)
		}
	}
	fmt.Fprint(w, style.Render(str))
}

type TopicsModel struct {
	List          list.Model
	Client        kafka.Cluster
	Ctx           context.Context
	LastSelected  string
	ConfirmDelete string
	ConfirmInput  textinput.Model
	Loading       bool
	Focused       bool
}

func NewTopicsModel(ctx context.Context, client kafka.Cluster) TopicsModel {
	l := list.New([]list.Item{}, topicDelegate{focused: true}, 0, 0)
	l.Title = "Kafka Topics"
	l.SetShowStatusBar(false)
	l.Styles.Title = components.GlobalTheme.TitleStyle
	
	ti := textinput.New()
	ti.Placeholder = "Type DELETE [topic] to confirm"
	ti.CharLimit = 100
	ti.Width = 40
	
	return TopicsModel{List: l, Client: client, Ctx: ctx, Loading: true, Focused: true, ConfirmInput: ti}
}

func (m *TopicsModel) Init() tea.Cmd { return m.Fetch() }

func (m *TopicsModel) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	if m.ConfirmDelete != "" {
		if km, ok := msg.(tea.KeyMsg); ok {
			if km.String() == "esc" {
				m.ConfirmDelete = ""
				m.ConfirmInput.Blur()
				return nil
			}
			if km.String() == "enter" {
				expected := m.ConfirmDelete
				if m.Client.IsProduction() {
					expected = "DELETE " + m.ConfirmDelete
				}
				
				if m.ConfirmInput.Value() == expected || (!m.Client.IsProduction() && m.ConfirmInput.Value() == "y") {
					if !m.Client.IsWriteEnabled() {
						m.ConfirmDelete = ""
						return func() tea.Msg { return common.ErrMsg{Err: fmt.Errorf("Action prohibited: client is in READ-ONLY mode")} }
					}
					topic := m.ConfirmDelete
					m.ConfirmDelete = ""
					m.ConfirmInput.Blur()
					items := m.List.Items()
					for i, it := range items {
						if it.(TopicListItem).Topic.Name == topic {
							li := it.(TopicListItem)
							li.Deleting = true
							m.List.SetItem(i, li)
							break
						}
					}
					common.AuditLog("DELETE_TOPIC", fmt.Sprintf("topic=%s", topic))
					return func() tea.Msg { return common.DeleteConfirmedMsg{Topic: topic} }
				}
			}
		}
		var cmd tea.Cmd
		m.ConfirmInput, cmd = m.ConfirmInput.Update(msg)
		return cmd // EXPLICIT RETURN: stop processing other keys
	}

	if m.Focused && m.List.FilterState() == 0 {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "n":
				if !m.Client.IsWriteEnabled() {
					return func() tea.Msg { return common.ErrMsg{Err: fmt.Errorf("Action prohibited: client is in READ-ONLY mode (use --write to enable)")} }
				}
				return func() tea.Msg { return common.CreateTopicMsg{} }
			case "d":
				if !m.Client.IsWriteEnabled() {
					return func() tea.Msg { return common.ErrMsg{Err: fmt.Errorf("Action prohibited: client is in READ-ONLY mode (use --write to enable)")} }
				}
				if i, ok := m.List.SelectedItem().(TopicListItem); ok {
					m.ConfirmDelete = i.Topic.Name
					m.ConfirmInput.Focus()
					m.ConfirmInput.SetValue("")
					if !m.Client.IsProduction() {
						m.ConfirmInput.Placeholder = "Type 'y' to confirm"
					} else {
						m.ConfirmInput.Placeholder = "Type DELETE " + i.Topic.Name
					}
					return nil
				}
			case "c":
				if i, ok := m.List.SelectedItem().(TopicListItem); ok {
					return func() tea.Msg { return common.ConsumeTopicMsg{Topic: i.Topic.Name, Partition: -1, Seek: kafka.SeekEnd} }
				}
			case "b":
				if i, ok := m.List.SelectedItem().(TopicListItem); ok {
					return func() tea.Msg { 
						return common.ConsumeTopicMsg{
							Topic: i.Topic.Name, 
							Partition: -1, 
							Seek: kafka.SeekLastN,
						} 
					}
				}
			case "p":
				if !m.Client.IsWriteEnabled() {
					return func() tea.Msg { return common.ErrMsg{Err: fmt.Errorf("Action prohibited: client is in READ-ONLY mode (use --write to enable)")} }
				}
				if i, ok := m.List.SelectedItem().(TopicListItem); ok {
					return func() tea.Msg { return common.ProduceTopicMsg{Topic: i.Topic.Name, Partition: -1} }
				}
			case "e":
				if !m.Client.IsWriteEnabled() {
					return func() tea.Msg { return common.ErrMsg{Err: fmt.Errorf("Action prohibited: client is in READ-ONLY mode (use --write to enable)")} }
				}
				if i, ok := m.List.SelectedItem().(TopicListItem); ok {
					return func() tea.Msg { return common.EditConfigMsg{Topic: i.Topic.Name} }
				}
			}
		}
	}

	switch msg := msg.(type) {
	case common.TopicsLoadedMsg:
		m.Loading = false
		items := make([]list.Item, len(msg.Topics))
		for i, t := range msg.Topics {
			items[i] = TopicListItem{Topic: t}
		}
		m.List.SetItems(items)
		m.syncSelection(&cmds)
	case common.TopicDeletedMsg:
		items := m.List.Items()
		for i, it := range items {
			if it.(TopicListItem).Topic.Name == msg.Topic {
				m.List.RemoveItem(i)
				break
			}
		}
	}

	var cmd tea.Cmd
	m.List, cmd = m.List.Update(msg)
	cmds = append(cmds, cmd)
	m.syncSelection(&cmds)
	return tea.Batch(cmds...)
}

func (m *TopicsModel) syncSelection(cmds *[]tea.Cmd) {
	if m.Loading || len(m.List.Items()) == 0 {
		return
	}
	if i, ok := m.List.SelectedItem().(TopicListItem); ok {
		if i.Topic.Name != m.LastSelected {
			m.LastSelected = i.Topic.Name
			*cmds = append(*cmds, func() tea.Msg { return common.TopicChangedMsg{Topic: m.LastSelected, Delayed: false} })
		}
	}
}

func (m *TopicsModel) HandleSize(w, h int) {
	m.List.SetSize(w, h)
}

func (m *TopicsModel) Close() {}

func (m *TopicsModel) Reset() {
	m.List.ResetFilter()
	m.ConfirmDelete = ""
}

func (m *TopicsModel) View() string {
	if m.Loading {
		return components.GlobalTheme.DimStyle.Render("Loading topics...")
	}

	if m.Focused {
		m.List.Styles.Title = components.GlobalTheme.TitleStyle
	} else {
		m.List.Styles.Title = components.GlobalTheme.InactiveTitleStyle
	}
	m.List.SetDelegate(topicDelegate{focused: m.Focused})

	view := m.List.View()
	if m.ConfirmDelete != "" {
		prompt := "Type 'y' to delete: "
		if m.Client.IsProduction() {
			prompt = fmt.Sprintf("PRODUCTION! Type 'DELETE %s' to confirm: ", m.ConfirmDelete)
		}
		
		confirm := components.GlobalTheme.ToastStyle.
			Render(prompt + m.ConfirmInput.View())
		return view + "\n" + confirm
	}
	return view
}

func (m *TopicsModel) Fetch() tea.Cmd {
	return func() tea.Msg {
		topics, err := m.Client.ListTopics(m.Ctx)
		if err != nil {
			return common.ErrMsg{Err: err}
		}
		return common.TopicsLoadedMsg{Topics: topics}
	}
}
