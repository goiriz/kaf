package pages

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/goiriz/kaf/internal/ui/common"
	"github.com/goiriz/kaf/internal/ui/components"
)

type HelpModel struct {
	Width, Height int
}

func NewHelpModel() HelpModel {
	return HelpModel{}
}

func (m HelpModel) Init() tea.Cmd { return nil }

func (m HelpModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "?":
			return func() tea.Msg { return common.BackMsg{} }
		}
	}
	return nil
}

func (m HelpModel) HandleSize(w, h int) {
	m.Width, m.Height = w, h
}

func (m HelpModel) Close() {}

func (m HelpModel) View() string {
	var s strings.Builder
	s.WriteString(components.GlobalTheme.TitleStyle.Render(" KAF - KEYBOARD SHORTCUTS ") + "\n\n")

	sections := []struct {
		title string
		keys  [][]string
	}{
		{"Global", [][]string{
			{"g", "Switch between Topics and Groups modes"},
			{"tab / arrows", "Change focus between Sidebar and Details"},
			{"C", "Switch Cluster Context"},
			{"?", "Show this help menu"},
			{"q / esc", "Go back or Quit"},
			{"ctrl+c", "Force Quit"},
		}},
		{"Topics List", [][]string{
			{"n", "Create a new topic (requires --write)"},
			{"d", "Delete selected topic (requires --write)"},
			{"c", "Tail topic (future messages only)"},
			{"b", "History mode (last 1000 messages)"},
			{"p", "Produce message to topic (requires --write)"},
			{"e", "Edit topic configuration (requires --write)"},
			{"/", "Filter list"},
		}},
		{"Consumer View", [][]string{
			{"up / down", "Select message"},
			{"y", "Copy message content to clipboard"},
			{"v", "Toggle Decoder (AUTO / HEX)"},
			{"/", "Search within messages (Press Enter to apply)"},
		}},
		{"Groups List", [][]string{
			{"r", "Reset offsets for selected group (requires --write)"},
			{"/", "Filter list"},
		}},
	}

	for _, sec := range sections {
		s.WriteString(components.GlobalTheme.SecondaryStyle.Render(" " + sec.title) + "\n")
		for _, k := range sec.keys {
			key := components.GlobalTheme.SelectedStyle.Render(fmt.Sprintf(" %-12s ", k[0]))
			s.WriteString(key + k[1] + "\n")
		}
		s.WriteString("\n")
	}

	return s.String()
}
