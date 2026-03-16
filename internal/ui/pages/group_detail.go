package pages

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui/common"
	"github.com/goiriz/kaf/internal/ui/components"
)

type GroupDetailModel struct {
	Client      kafka.Cluster
	Ctx         context.Context
	GroupID     string
	Detail      *kafka.GroupDetail
	Cursor      int
	Focused     bool
	Loading     bool
	SearchInput textinput.Model
	Searching   bool
	Width       int
	Height      int
}

func NewGroupDetailModel(ctx context.Context, client kafka.Cluster, groupID string) GroupDetailModel {
	ti := textinput.New()
	ti.Placeholder = "Filter offsets (topic or partition)..."
	ti.Prompt = " / "
	return GroupDetailModel{
		Client:      client,
		Ctx:         ctx,
		GroupID:     groupID,
		Loading:     true,
		SearchInput: ti,
	}
}

func (m *GroupDetailModel) Init() tea.Cmd {
	return m.Fetch()
}

func (m *GroupDetailModel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	if m.Searching {
		if km, ok := msg.(tea.KeyMsg); ok {
			if km.String() == "enter" || km.String() == "esc" {
				m.Searching = false
				m.SearchInput.Blur()
				return nil
			}
		}
		m.SearchInput, cmd = m.SearchInput.Update(msg)
		return cmd
	}

	switch msg := msg.(type) {
	case common.GroupDetailLoadedMsg:
		m.Loading = false
		m.Detail = msg.Detail
		return nil

	case tea.KeyMsg:
		switch msg.String() {
		case "/":
			m.Searching = true
			m.SearchInput.Focus()
			return nil
		case "up", "k":
			if m.Focused && m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			m.Cursor++
		case "r":
			if !m.Client.IsWriteEnabled() {
				return func() tea.Msg { return common.ErrMsg{Err: fmt.Errorf("Action prohibited: client is in READ-ONLY mode (use --write to enable)")} }
			}
			o := m.getSelectedOffset()
			if o != nil {
				return func() tea.Msg {
					return common.ResetOffsetMsg{
						GroupID:    m.GroupID,
						GroupState: m.Detail.State,
						Topic:      o.Topic,
						Partition:  o.Partition,
						Current:    o.GroupOffset,
					}
				}
			}
		case "c":
			o := m.getSelectedOffset()
			if o != nil {
				return func() tea.Msg { return common.ConsumeTopicMsg{Topic: o.Topic, Partition: o.Partition, Seek: kafka.SeekEnd} }
			}
		case "b":
			o := m.getSelectedOffset()
			if o != nil {
				return func() tea.Msg { return common.ConsumeTopicMsg{Topic: o.Topic, Partition: o.Partition, Seek: kafka.SeekLastN} }
			}
		}
	}
	return nil
}

func (m *GroupDetailModel) getSelectedOffset() *kafka.GroupOffset {
	if m.Detail == nil {
		return nil
	}
	filter := strings.ToLower(m.SearchInput.Value())
	var filtered []kafka.GroupOffset
	for _, o := range m.Detail.Offsets {
		if filter == "" || common.FilterMatches(o.Topic, filter) || common.FilterMatches(strconv.Itoa(o.Partition), filter) {
			filtered = append(filtered, o)
		}
	}
	if len(filtered) > 0 && m.Cursor >= 0 && m.Cursor < len(filtered) {
		return &filtered[m.Cursor]
	}
	return nil
}

func (m *GroupDetailModel) HandleSize(w, h int) {
	m.Width, m.Height = w, h
}

func (m *GroupDetailModel) Close() {}

func (m *GroupDetailModel) View() string {
	if m.Loading {
		return components.GlobalTheme.DimStyle.Render("Loading group details...")
	}
	if m.Detail == nil {
		return ""
	}

	// Filter offsets
	filter := strings.ToLower(m.SearchInput.Value())
	var filtered []kafka.GroupOffset
	for _, o := range m.Detail.Offsets {
		if filter == "" ||
			common.FilterMatches(o.Topic, filter) ||
			common.FilterMatches(strconv.Itoa(o.Partition), filter) {
			filtered = append(filtered, o)
		}
	}

	if m.Cursor >= len(filtered) {
		m.Cursor = 0
		if len(filtered) == 0 {
			m.Cursor = -1
		}
	} else if m.Cursor < 0 && len(filtered) > 0 {
		m.Cursor = 0
	}

	var s strings.Builder

	// Header
	titleStr := "GROUP: " + m.Detail.ID
	style := components.GlobalTheme.InactiveTitleStyle
	if m.Focused {
		style = components.GlobalTheme.TitleStyle
	}
	s.WriteString(style.Render(titleStr) + "\n")
	s.WriteString(fmt.Sprintf("State: %s\n\n", m.Detail.State))

	if m.Searching {
		s.WriteString(m.SearchInput.View() + "\n\n")
	}

	s.WriteString(components.TableHeader(fmt.Sprintf("  %-20s %-5s %-10s %-10s %-8s", "TOPIC", "PART", "COMMITTED", "LATEST", "LAG")) + "\n")
	s.WriteString("  " + strings.Repeat("─", 55) + "\n")

	for i, o := range filtered {
		lagStr := fmt.Sprintf("%d", o.Lag)
		if o.Lag > 0 {
			lagStr = components.GlobalTheme.ErrorStyle.Copy().Bold(true).Render(lagStr)
		}

		cursor := " "
		rowStyle := lipgloss.NewStyle()
		if i == m.Cursor {
			cursor = ">"
			if m.Focused {
				rowStyle = components.GlobalTheme.SelectedStyle
			} else {
				rowStyle = components.GlobalTheme.SecondaryStyle.Copy().Bold(false)
			}
		}

		s.WriteString(rowStyle.Render(fmt.Sprintf("%s %-20s %-5d %-10d %-10d %-8s", cursor, o.Topic, o.Partition, o.GroupOffset, o.LatestOffset, lagStr)) + "\n")
	}

	if len(filtered) == 0 {
		s.WriteString(components.GlobalTheme.DimStyle.Italic(true).Render("  (No offsets match filter)\n"))
	}

	s.WriteString("\n" + lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, false, false).BorderForeground(components.GlobalTheme.DimColor).Render(
		fmt.Sprintf(" Total Lag: %s  |  Showing: %d/%d",
			components.SelectedStyle.Render(fmt.Sprintf("%d", m.Detail.TotalLag)),
			len(filtered), len(m.Detail.Offsets)),
	))

	return s.String()
}

func (m *GroupDetailModel) Fetch() tea.Cmd {
	m.Loading = true
	return func() tea.Msg {
		detail, err := m.Client.GetGroupDetail(m.Ctx, m.GroupID)
		if err != nil {
			return common.ErrMsg{Err: err}
		}
		return common.GroupDetailLoadedMsg{Detail: detail}
	}
}
