package pages

import (
	"context"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui/common"
	"github.com/goiriz/kaf/internal/ui/components"
)

type TopicCreateModel struct {
	Client     kafka.Cluster
	Ctx        context.Context
	Inputs     []textinput.Model
	FocusIndex int
	Err        error
}

func NewTopicCreateModel(ctx context.Context, client kafka.Cluster) TopicCreateModel {
	n := textinput.New()
	n.Placeholder = "Name"
	n.Prompt = " NAME: "
	n.Focus()
	p := textinput.New()
	p.Placeholder = "1"
	p.Prompt = " PARTITIONS: "
	r := textinput.New()
	r.Placeholder = "1"
	r.Prompt = " REPLICATION: "
	return TopicCreateModel{Client: client, Ctx: ctx, Inputs: []textinput.Model{n, p, r}}
}

func (m *TopicCreateModel) Init() tea.Cmd { return textinput.Blink }

func (m *TopicCreateModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "up", "down":
			m.Inputs[m.FocusIndex].Blur()
			if msg.String() == "up" || msg.String() == "shift+tab" {
				m.FocusIndex = (m.FocusIndex - 1 + len(m.Inputs)) % len(m.Inputs)
			} else {
				m.FocusIndex = (m.FocusIndex + 1) % len(m.Inputs)
			}
			m.Inputs[m.FocusIndex].Focus()
			return nil
		case "enter":
			p, _ := strconv.Atoi(m.Inputs[1].Value())
			if p <= 0 {
				p = 1
			}
			r, _ := strconv.Atoi(m.Inputs[2].Value())
			if r <= 0 {
				r = 1
			}
			return m.Submit(m.Inputs[0].Value(), p, r)
		}
	}
	var cmd tea.Cmd
	m.Inputs[m.FocusIndex], cmd = m.Inputs[m.FocusIndex].Update(msg)
	return cmd
}

func (m *TopicCreateModel) HandleSize(w, h int) {
	// Simple form, doesn't need specific resizing logic for now
}


func (m *TopicCreateModel) View() string {
	var s strings.Builder
	s.WriteString(components.GlobalTheme.SecondaryStyle.Render("CREATE NEW TOPIC") + "\n\n")
	for i := range m.Inputs {
		s.WriteString(m.Inputs[i].View() + "\n")
	}
	if m.Err != nil {
		s.WriteString("\n" + components.GlobalTheme.ErrorStyle.Render(m.Err.Error()) + "\n")
	}
	s.WriteString("\n" + components.HelpStyle.Render("(esc: cancel, enter: create)"))
	return s.String()
}

func (m *TopicCreateModel) Submit(name string, p, r int) tea.Cmd {
	return func() tea.Msg {
		if err := m.Client.CreateTopic(m.Ctx, name, p, r); err != nil {
			return common.ErrMsg{Err: err}
		}
		return common.TopicCreatedMsg{}
	}
}


func (m *TopicCreateModel) Close() {}
