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

type roState int

const (
	roStateInput roState = iota
	roStateConfirm
)

type ResetOffsetModel struct {
	Client     kafka.Cluster
	Ctx        context.Context
	GroupID    string
	GroupState string
	Topic      string
	Partition  int
	Current    int64
	Min, Max   int64
	Inputs     []textinput.Model
	Focus      int
	Err        error

	// Confirmation
	TargetOffset int64
	State        roState
}

func NewResetOffsetModel(ctx context.Context, client kafka.Cluster, groupID, groupState, topic string, partition int, current int64) ResetOffsetModel {
	m := ResetOffsetModel{
		Client:     client,
		Ctx:        ctx,
		GroupID:    groupID,
		GroupState: groupState,
		Topic:      topic,
		Partition:  partition,
		Current:    current,
		Inputs:     make([]textinput.Model, 2),
		State:      roStateInput,
	}

	m.Inputs[0] = textinput.New()
	m.Inputs[0].Placeholder = "Offset, 'earliest' or 'latest'"
	m.Inputs[0].Focus()
	m.Inputs[0].Prompt = "> "

	m.Inputs[1] = textinput.New()
	if client.IsProduction() {
		m.Inputs[1].Placeholder = "Type RESET " + groupID + " to confirm"
	} else {
		m.Inputs[1].Placeholder = "Type 'RESET' to confirm"
	}
	m.Inputs[1].Prompt = "> "
	m.Inputs[1].CharLimit = 100
	m.Inputs[1].Width = 40

	return m
}

func (m *ResetOffsetModel) Init() tea.Cmd {
	// Fetch topic detail to get current bounds
	return tea.Batch(textinput.Blink, func() tea.Msg {
		detail, err := m.Client.GetTopicDetail(m.Ctx, m.Topic)
		if err != nil {
			return common.ErrMsg{Err: err}
		}
		return common.TopicDetailLoadedMsg{Detail: detail}
	})
}

func (m *ResetOffsetModel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	if m.State == roStateConfirm {
		if km, ok := msg.(tea.KeyMsg); ok {
			if km.String() == "esc" {
				m.State = roStateInput
				m.Inputs[0].Focus()
				m.Inputs[1].SetValue("")
				m.Err = nil
				return nil
			}
			if km.String() == "enter" {
				expected := "RESET"
				if m.Client.IsProduction() {
					expected = "RESET " + m.GroupID
				}
				
				if strings.ToUpper(m.Inputs[1].Value()) == expected {
					return m.Submit(m.TargetOffset)
				}
				m.Err = fmt.Errorf("confirmation failed: please type %s", expected)
				return nil
			}
		}
		m.Inputs[1], cmd = m.Inputs[1].Update(msg)
		return cmd
	}

	switch msg := msg.(type) {
	case common.TopicDetailLoadedMsg:
		for _, p := range msg.Detail.Partitions {
			if p.ID == m.Partition {
				m.Min, m.Max = p.StartOffset, p.EndOffset
				break
			}
		}
		return nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return func() tea.Msg { return common.BackMsg{} }
		case "enter":
			val := strings.ToLower(strings.TrimSpace(m.Inputs[0].Value()))
			var target int64
			var err error

			switch val {
			case "earliest", "e":
				target = m.Min
			case "latest", "l":
				target = m.Max
			default:
				cleanVal := strings.ReplaceAll(val, ",", "")
				cleanVal = strings.ReplaceAll(cleanVal, " ", "")
				target, err = strconv.ParseInt(cleanVal, 10, 64)
			}

			if err != nil {
				m.Err = fmt.Errorf("invalid offset: %s", val)
				return nil
			}

			if target < m.Min || target > m.Max {
				m.Err = fmt.Errorf("out of bounds: %d", target)
				return nil
			}

			m.TargetOffset = target
			m.State = roStateConfirm
			m.Inputs[1].Focus()
			m.Err = nil
			return nil
		}
	}

	m.Inputs[0], cmd = m.Inputs[0].Update(msg)
	return cmd
}

func (m *ResetOffsetModel) HandleSize(w, h int) {
	// Simple form
}

func (m *ResetOffsetModel) Close() {}

func (m *ResetOffsetModel) View() string {
	var s strings.Builder
	s.WriteString(components.GlobalTheme.SecondaryStyle.Render(fmt.Sprintf("RESET OFFSET - Group: %s, Topic: %s, Part: %d", m.GroupID, m.Topic, m.Partition)) + "\n\n")

	if m.GroupState != "Empty" && m.GroupState != "Dead" {
		warn := lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("208")).Padding(0, 1).Bold(true).
			Render(fmt.Sprintf(" WARNING: Group state is '%s'. Stop all consumers before resetting! ", m.GroupState))
		s.WriteString(warn + "\n\n")
	}

	if m.State == roStateInput {
		if m.Max > 0 || m.Min > 0 {
			s.WriteString(components.GlobalTheme.DimStyle.Render(fmt.Sprintf("Available range: [%d - %d]  |  Current offset: %d", m.Min, m.Max, m.Current)) + "\n\n")
		}
		s.WriteString(m.Inputs[0].View() + "\n\n")
		s.WriteString(components.GlobalTheme.DimStyle.Render("Enter number, 'e' for earliest, or 'l' for latest."))
	} else {
		diff := m.TargetOffset - m.Current
		diffStr := fmt.Sprintf("%+d", diff)
		if diff < 0 {
			diffStr = components.GlobalTheme.ErrorStyle.Render(diffStr)
		} else {
			diffStr = components.GlobalTheme.InfoStyle.Render(diffStr)
		}

		s.WriteString(fmt.Sprintf("Target Offset: %d (Change: %s)\n\n", m.TargetOffset, diffStr))
		s.WriteString(m.Inputs[1].View() + "\n\n")
		s.WriteString(components.GlobalTheme.ErrorStyle.Copy().Bold(true).Render("DANGEROUS ACTION: Type 'RESET' and press Enter to apply."))
	}

	if m.Err != nil {
		s.WriteString("\n\n" + components.GlobalTheme.ErrorStyle.Render(m.Err.Error()))
	}

	return s.String()
}

func (m *ResetOffsetModel) Submit(offset int64) tea.Cmd {
	return func() tea.Msg {
		if !m.Client.IsWriteEnabled() {
			return common.ErrMsg{Err: kafka.ErrReadOnly}
		}
		common.AuditLog("RESET_OFFSET", fmt.Sprintf("group=%s topic=%s partition=%d target_offset=%d current_offset=%d", m.GroupID, m.Topic, m.Partition, offset, m.Current))
		if err := m.Client.CommitOffset(m.Ctx, m.GroupID, m.Topic, m.Partition, offset); err != nil {
			return common.ErrMsg{Err: err}
		}
		return common.OffsetResetSuccessMsg{GroupID: m.GroupID}
	}
}
