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

type TopicDetailModel struct {
	Client           kafka.Cluster
	Ctx              context.Context
	Topic            string
	Detail           *kafka.TopicDetail
	Cursor           int
	ScrollOffset     int
	Width            int
	Height           int
	Loading          bool
	Focused          bool
	TotalPartitions  int
	LoadedPartitions int

	SearchInput textinput.Model
	Searching   bool
}

func NewTopicDetailModel(ctx context.Context, client kafka.Cluster, topic string) TopicDetailModel {
	ti := textinput.New()
	ti.Placeholder = "Filter by ID or Leader..."
	ti.Prompt = " / "
	return TopicDetailModel{Client: client, Ctx: ctx, Topic: topic, Loading: true, SearchInput: ti}
}

func (m *TopicDetailModel) Init() tea.Cmd {
	if m.Topic == "" {
		return nil
	}
	return m.Fetch()
}

func (m *TopicDetailModel) Update(msg tea.Msg) tea.Cmd {
	if m.Searching {
		if km, ok := msg.(tea.KeyMsg); ok && (km.String() == "enter" || km.String() == "esc") {
			m.Searching = false
			m.SearchInput.Blur()
			return nil
		}
		var cmd tea.Cmd
		m.SearchInput, cmd = m.SearchInput.Update(msg)
		return cmd
	}

	switch msg := msg.(type) {
	case common.TopicDetailLoadedMsg:
		m.Loading = false
		m.Detail = msg.Detail
		return nil

	case tea.KeyMsg:
		switch msg.String() {
		case "/":
			m.Searching = true
			m.SearchInput.Focus()
			m.SearchInput.SetValue("")
			return nil
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
				if m.Cursor < m.ScrollOffset {
					m.ScrollOffset = m.Cursor
				}
			}
		case "down", "j":
			// We handle max bounds in View after filtering
			m.Cursor++
		case "c":
			if m.Detail != nil && len(m.Detail.Partitions) > 0 {
				pID := m.Detail.Partitions[m.Cursor].ID
				return func() tea.Msg { return common.ConsumeTopicMsg{Topic: m.Topic, Partition: pID, Seek: kafka.SeekEnd} }
			}
		case "b":
			if m.Detail != nil && len(m.Detail.Partitions) > 0 {
				pID := m.Detail.Partitions[m.Cursor].ID
				return func() tea.Msg { 
					return common.ConsumeTopicMsg{
						Topic: m.Topic, 
						Partition: pID, 
						Seek: kafka.SeekLastN,
					} 
				}
			}
		case "p":
			if !m.Client.IsWriteEnabled() {
				return func() tea.Msg { return common.ErrMsg{Err: fmt.Errorf("Action prohibited: client is in READ-ONLY mode (use --write to enable)")} }
			}
			if m.Detail != nil && len(m.Detail.Partitions) > 0 {
				pID := m.Detail.Partitions[m.Cursor].ID
				return func() tea.Msg { return common.ProduceTopicMsg{Topic: m.Topic, Partition: pID} }
			}
		}
	}
	return nil
}

func (m *TopicDetailModel) HandleSize(w, h int) {
	m.Width, m.Height = w, h
}

func (m *TopicDetailModel) Close() {}

func (m *TopicDetailModel) View() string {
	if m.Loading {
		return components.GlobalTheme.DimStyle.Render("Loading topic details...")
	}
	if m.Detail == nil {
		return "Select a topic"
	}

	// Filter partitions
	filter := strings.ToLower(m.SearchInput.Value())
	var filtered []kafka.PartitionDetail
	for _, p := range m.Detail.Partitions {
		if filter == "" ||
			common.FilterMatches(strconv.Itoa(p.ID), filter) ||
			common.FilterMatches(p.Leader, filter) {
			filtered = append(filtered, p)
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
	titleStr := "TOPIC: " + m.Detail.Name
	style := components.InactiveTitleStyle
	if m.Focused {
		style = components.TitleStyle
	}
	s.WriteString(style.Render(titleStr) + "\n\n")

	if m.Searching {
		s.WriteString(m.SearchInput.View() + "\n\n")
	}

	// Table Header
	s.WriteString(components.TableHeader(fmt.Sprintf("  %-4s %-12s %-12s %-10s", "ID", "Start", "End", "Count")) + "\n")
	s.WriteString("  " + strings.Repeat("─", 45) + "\n")

	visibleRows := m.Height - 10
	if m.Searching {
		visibleRows -= 2
	}
	if visibleRows < 1 {
		visibleRows = 1
	}

	if m.Cursor >= 0 && m.Cursor >= m.ScrollOffset+visibleRows {
		m.ScrollOffset = m.Cursor - visibleRows + 1
	}
	if m.Cursor >= 0 && m.Cursor < m.ScrollOffset {
		m.ScrollOffset = m.Cursor
	}

	end := m.ScrollOffset + visibleRows
	if end > len(filtered) {
		end = len(filtered)
	}

	var totalMessages int64
	for i, p := range filtered {
		count := p.EndOffset - p.StartOffset
		totalMessages += count
		if i >= m.ScrollOffset && i < end {
			cursor, rowStyle := " ", lipgloss.NewStyle()
			if m.Cursor == i {
				cursor = ">"
				if m.Focused {
					rowStyle = components.SelectedStyle
				} else {
					rowStyle = components.GlobalTheme.SecondaryStyle.Copy().Bold(false)
				}
			}
			s.WriteString(rowStyle.Render(fmt.Sprintf("%s %-4d %-12d %-12d %-10d", cursor, p.ID, p.StartOffset, p.EndOffset, count)) + "\n")
		}
	}

	if end < len(filtered) {
		s.WriteString(components.GlobalTheme.DimStyle.Render(fmt.Sprintf("  ... %d more", len(filtered)-end)) + "\n")
	} else if len(filtered) == 0 {
		s.WriteString(components.GlobalTheme.DimStyle.Render("  (No partitions match filter)\n"))
	} else {
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, false, false).BorderForeground(components.GlobalTheme.DimColor).Render(
		fmt.Sprintf(" Total Messages: %s  |  Partitions: %d/%d",
			components.SelectedStyle.Render(fmt.Sprintf("%d", totalMessages)),
			len(filtered), len(m.Detail.Partitions)),
	))

	return s.String()
}

func (m *TopicDetailModel) Fetch() tea.Cmd {
	return func() tea.Msg {
		detail, err := m.Client.GetTopicDetail(m.Ctx, m.Topic)
		if err != nil {
			return common.ErrMsg{Err: err}
		}
		return common.TopicDetailLoadedMsg{Detail: detail}
	}
}
