package pages

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui/common"
	"github.com/goiriz/kaf/internal/ui/components"
)

type TopicProduceModel struct {
	Client    kafka.Cluster
	Ctx       context.Context
	Topic     string
	Partition int
	Inputs    []textinput.Model
	Focus     int
	Err       error

	Confirming bool
}

func NewTopicProduceModel(ctx context.Context, client kafka.Cluster, topic string, partition int) TopicProduceModel {
	inputs := make([]textinput.Model, 4)
	
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Key (optional)"
	inputs[0].Focus()
	inputs[0].Prompt = " Key     > "

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "Value"
	inputs[1].Prompt = " Value   > "

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "Headers (e.g. k1:v1,k2:v2)"
	inputs[2].Prompt = " Headers > "

	inputs[3] = textinput.New()
	inputs[3].Placeholder = "Type 'PRODUCE' to confirm"
	inputs[3].Prompt = " Confirm > "

	return TopicProduceModel{
		Client:    client,
		Ctx:       ctx,
		Topic:     topic,
		Partition: partition,
		Inputs:    inputs,
	}
}

func (m *TopicProduceModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *TopicProduceModel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	if m.Confirming {
		if km, ok := msg.(tea.KeyMsg); ok {
			if km.String() == "esc" {
				m.Confirming = false
				m.Focus = 1
				m.Inputs[1].Focus()
				return nil
			}
			if km.String() == "enter" {
				if strings.ToUpper(m.Inputs[3].Value()) == "PRODUCE" {
					return m.Submit()
				}
				m.Err = fmt.Errorf("confirmation failed: type 'PRODUCE'")
				return nil
			}
		}
		m.Inputs[3], cmd = m.Inputs[3].Update(msg)
		return cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return func() tea.Msg { return common.BackMsg{} }
		case "tab", "shift+tab", "up", "down":
			s := msg.String()
			if s == "up" || s == "shift+tab" {
				m.Focus--
			} else {
				m.Focus++
			}

			// We skip the hidden confirmation field in input mode
			if m.Focus < 0 {
				m.Focus = 2
			} else if m.Focus >= 3 {
				m.Focus = 0
			}

			cmds := make([]tea.Cmd, 3)
			for i := 0; i <= 2; i++ {
				if i == m.Focus {
					cmds[i] = m.Inputs[i].Focus()
					continue
				}
				m.Inputs[i].Blur()
			}
			return tea.Batch(cmds...)

		case "enter":
			if m.Client.IsProduction() {
				m.Confirming = true
				m.Inputs[3].Focus()
				m.Inputs[3].SetValue("")
				m.Err = nil
				return nil
			}
			if m.Focus == 2 || m.Inputs[1].Value() != "" {
				return m.Submit()
			}
			m.Focus++
			return m.Inputs[m.Focus].Focus()
		}
	}

	m.Inputs[m.Focus], cmd = m.Inputs[m.Focus].Update(msg)
	return cmd
}

func (m *TopicProduceModel) HandleSize(w, h int) {}
func (m *TopicProduceModel) Close()              {}

func (m *TopicProduceModel) View() string {
	var s strings.Builder
	title := fmt.Sprintf("PRODUCE MESSAGE - Topic: %s", m.Topic)
	if m.Partition >= 0 {
		title += fmt.Sprintf(" (P:%d)", m.Partition)
	}
	s.WriteString(components.GlobalTheme.SecondaryStyle.Render(title) + "\n\n")

	if m.Confirming {
		s.WriteString(components.GlobalTheme.ToastStyle.Render(" PRODUCTION ALERT: About to push to production topic! ") + "\n\n")
		s.WriteString(fmt.Sprintf(" Key: %s\n Value: %s\n Headers: %s\n\n", m.Inputs[0].Value(), m.Inputs[1].Value(), m.Inputs[2].Value()))
		s.WriteString(m.Inputs[3].View() + "\n")
		s.WriteString(components.GlobalTheme.ErrorStyle.Render("\n[Esc] cancel • [Enter] confirm and produce"))
	} else {
		for i := 0; i <= 2; i++ {
			s.WriteString(m.Inputs[i].View() + "\n")
		}
		s.WriteString(components.GlobalTheme.DimStyle.Render("\n[Tab] switch fields • [Enter] produce • [Esc] back"))
	}

	if m.Err != nil {
		s.WriteString("\n\n" + components.GlobalTheme.ErrorStyle.Render(m.Err.Error()))
	}

	return s.String()
}


func (m *TopicProduceModel) Submit() tea.Cmd {
	key := []byte(m.Inputs[0].Value())
	value := []byte(m.Inputs[1].Value())
	headerStr := m.Inputs[2].Value()
	
	headers := make(map[string]string)
	if headerStr != "" {
		pairs := strings.Split(headerStr, ",")
		for _, p := range pairs {
			kv := strings.SplitN(p, ":", 2)
			if len(kv) == 2 {
				headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
	}

	return func() tea.Msg {
		common.AuditLog("PRODUCE", fmt.Sprintf("topic=%s partition=%d key_len=%d val_len=%d", m.Topic, m.Partition, len(key), len(value)))
		err := m.Client.Produce(m.Ctx, m.Topic, m.Partition, key, value, headers)
		if err != nil {
			return common.ErrMsg{Err: err}
		}
		return common.InfoMsg{Text: "Message produced successfully"}
	}
}
