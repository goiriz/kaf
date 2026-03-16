package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/goiriz/kaf/internal/config"
	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui/common"
	"github.com/goiriz/kaf/internal/ui/components"
	"github.com/goiriz/kaf/internal/ui/pages"
)

type state int

const (
	stateMain state = iota
	stateTopicCreate
	stateTopicConsume
	stateTopicProduce
	stateTopicConfig
	stateResetOffset
	stateHelp
	stateContexts
)

type mode int

const (
	modeTopics mode = iota
	modeGroups
)

type focus int

const (
	focusSidebar focus = iota
	focusDetails
)

type Model struct {
	state     state
	mode      mode
	prevState state
	focus     focus
	client    kafka.Cluster
	cfg       *config.Config

	// Request management
	ctx           context.Context
	cancelGlobal  context.CancelFunc
	cancelDetails context.CancelFunc

	// Main Split View Components
	topics pages.TopicsModel
	groups pages.GroupsModel
	topicDetail *pages.TopicDetailModel
	groupDetail *pages.GroupDetailModel

	// Unified Active Page (for non-main states)
	activePage common.Page

	width, height int
	quitting      bool
	
	// Toast notifications
	toastMsg     string
	toastTime    time.Time
	toastIsError bool
}

func NewModel(cfg *config.Config, client kafka.Cluster) Model {
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		state:        stateMain,
		mode:         modeTopics,
		focus:        focusSidebar,
		client:       client,
		cfg:          cfg,
		ctx:          ctx,
		cancelGlobal: cancel,
		topics:       pages.NewTopicsModel(ctx, client),
		groups:       pages.NewGroupsModel(ctx, client),
	}
}

func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, m.topics.Init())
	
	// Check for decoder errors (e.g. proto parsing failures)
	if errs := m.client.GetDecoder().Errors(); len(errs) > 0 {
		cmds = append(cmds, func() tea.Msg {
			return common.ErrMsg{Err: fmt.Errorf("%s", errs[0])}
		})
	}
	
	return tea.Batch(cmds...)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if km, ok := msg.(tea.KeyMsg); ok {
		if res, cmd := m.handleGlobalKeys(km); res != nil {
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return res, tea.Batch(cmds...)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.updateLayout()
	case common.ErrMsg:
		common.Log("Error: %v", msg.Err)
		m.toastMsg = msg.Err.Error()
		m.toastIsError = true
		m.toastTime = time.Now()
		return m, tea.Tick(4*time.Second, func(t time.Time) tea.Msg {
			return common.ClearToastMsg{Time: m.toastTime}
		})
	case common.InfoMsg:
		m.toastMsg = msg.Text
		m.toastIsError = false
		m.toastTime = time.Now()
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return common.ClearToastMsg{Time: m.toastTime}
		})
	case common.ClearToastMsg:
		if msg.Time.Equal(m.toastTime) {
			m.toastMsg = ""
		}
		return m, nil
	case common.TopicChangedMsg:
		if msg.Delayed {
			if m.topicDetail != nil && m.topicDetail.Topic == msg.Topic {
				return m, m.topicDetail.Init()
			}
			return m, nil
		}
		return m, m.setTopic(msg.Topic)
	case common.GroupChangedMsg:
		if msg.Delayed {
			if m.groupDetail != nil && m.groupDetail.GroupID == msg.GroupID {
				return m, m.groupDetail.Init()
			}
			return m, nil
		}
		return m, m.setGroup(msg.GroupID)
	case common.CreateTopicMsg:
		return m.startCreate(), nil
	case common.DeleteConfirmedMsg:
		cmds = append(cmds, m.deleteTopic(msg.Topic))
	case common.TopicDeletedMsg:
		cmds = append(cmds, tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg { return common.RefreshTopicsMsg{} }))
	case common.TopicCreatedMsg:
		m.state = stateMain
		m.activePage = nil
		cmds = append(cmds, tea.Tick(1*time.Second, func(t time.Time) tea.Msg { return common.RefreshTopicsMsg{} }))
	case common.RefreshTopicsMsg:
		cmds = append(cmds, m.topics.Init())
	case common.ConsumeTopicMsg:
		return m, m.startConsume(msg)
	case common.ProduceTopicMsg:
		return m, m.startProduce(msg)
	case common.EditConfigMsg:
		return m, m.startConfig(msg)
	case common.ResetOffsetMsg:
		return m, m.startResetOffset(msg)
	case common.OffsetResetSuccessMsg:
		m.state = stateMain
		m.activePage = nil
		m.resetMainState()
		if m.groupDetail != nil && m.groupDetail.GroupID == msg.GroupID {
			return m, m.groupDetail.Fetch()
		}
		return m, nil
	case common.BackMsg:
		m.state = stateMain
		m.activePage = nil
		m.resetMainState()
		return m, nil
	case common.SwitchContextMsg:
		newCtx, ok := m.cfg.Contexts[msg.ContextName]
		if !ok {
			return m, func() tea.Msg { return common.ErrMsg{Err: fmt.Errorf("context not found")} }
		}
		
		// Clean up old client and cancel ongoing detail requests
		m.client.Close()
		if m.cancelDetails != nil {
			m.cancelDetails()
		}

		// Initialize new client and reset UI
		m.client = kafka.NewClient(&newCtx)
		m.cfg.CurrentContext = msg.ContextName // In-memory update
		
		m.topics = pages.NewTopicsModel(m.ctx, m.client)
		m.groups = pages.NewGroupsModel(m.ctx, m.client)
		m.topics.HandleSize(m.width*40/100-2, m.height-5)
		m.groups.HandleSize(m.width*40/100-2, m.height-5)
		m.topicDetail = nil
		m.groupDetail = nil
		
		m.state = stateMain
		m.activePage = nil
		m.resetMainState()
		
		return m, tea.Batch(
			m.topics.Init(),
			func() tea.Msg { return common.InfoMsg{Text: "Switched to cluster: " + msg.ContextName} },
		)
	}

	m.delegateUpdate(msg, &cmds)
	return m, tea.Batch(cmds...)
}

func (m *Model) handleGlobalKeys(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	if km.String() == "ctrl+c" {
		m.cancelGlobal()
		m.quitting = true
		return m, tea.Quit
	}

	switch km.String() {
	case "q":
		if m.state == stateMain && !m.isFiltering() {
			m.cancelGlobal()
			m.quitting = true
			return m, tea.Quit
		}
		if m.state != stateMain {
			if m.activePage != nil { m.activePage.Close() }
			m.state = stateMain
			m.activePage = nil
			m.resetMainState()
			return m, nil
		}
	case "esc":
		if m.state != stateMain {
			if m.activePage != nil { m.activePage.Close() }
			m.state = stateMain
			m.activePage = nil
			m.resetMainState()
			return m, nil
		}
		if m.focus == focusDetails {
			m.focus = focusSidebar
			m.syncFocus()
			return m, nil
		}
		if m.mode == modeGroups {
			return m, m.toggleMode()
		}
		m.cancelGlobal()
		m.quitting = true
		return m, tea.Quit
	case "tab", "right":
		if m.state == stateMain && m.focus == focusSidebar && !m.isFiltering() {
			m.focus = focusDetails
			m.syncFocus()
			return m, nil
		}
	case "left":
		if m.state == stateMain && m.focus == focusDetails {
			m.focus = focusSidebar
			m.syncFocus()
			return m, nil
		}
	case "g":
		if m.state == stateMain && !m.isFiltering() {
			return m, m.toggleMode()
		}
	case "?":
		if m.state == stateMain && !m.isFiltering() {
			m.prevState = m.state
			m.state = stateHelp
			h := pages.NewHelpModel()
			m.activePage = &h
			return m, nil
		}
	case "C":
		if m.state == stateMain && !m.isFiltering() && m.cfg != nil {
			m.prevState = m.state
			m.state = stateContexts
			c := pages.NewContextsModel(m.cfg)
			m.activePage = &c
			return m, nil
		}
	}
	return nil, nil
}

func (m *Model) resetMainState() {
	m.focus = focusSidebar
	m.topics.Reset()
	m.groups.Reset()
	m.syncFocus()
}

func (m *Model) syncFocus() {
	m.topics.Focused = (m.focus == focusSidebar)
	m.groups.Focused = (m.focus == focusSidebar)
	if m.topicDetail != nil {
		m.topicDetail.Focused = (m.focus == focusDetails)
	}
	if m.groupDetail != nil {
		m.groupDetail.Focused = (m.focus == focusDetails)
	}
}

func (m *Model) toggleMode() tea.Cmd {
	m.focus = focusSidebar
	m.syncFocus()
	if m.mode == modeTopics {
		m.mode = modeGroups
		return m.groups.Init()
	}
	m.mode = modeTopics
	return m.topics.Init()
}

func (m *Model) setTopic(topic string) tea.Cmd {
	if topic == "" {
		m.topicDetail = nil
		return nil
	}

	if m.cancelDetails != nil {
		m.cancelDetails()
	}

	var ctx context.Context
	ctx, m.cancelDetails = context.WithCancel(m.ctx)

	td := pages.NewTopicDetailModel(ctx, m.client, topic)
	m.topicDetail = &td
	m.updateLayout()

	return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
		select {
		case <-ctx.Done():
			return nil
		default:
			return common.TopicChangedMsg{Topic: topic, Delayed: true}
		}
	})
}

func (m *Model) setGroup(groupID string) tea.Cmd {
	if groupID == "" {
		m.groupDetail = nil
		return nil
	}

	if m.cancelDetails != nil {
		m.cancelDetails()
	}

	var ctx context.Context
	ctx, m.cancelDetails = context.WithCancel(m.ctx)

	gd := pages.NewGroupDetailModel(ctx, m.client, groupID)
	m.groupDetail = &gd
	m.updateLayout()

	return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
		select {
		case <-ctx.Done():
			return nil
		default:
			return common.GroupChangedMsg{GroupID: groupID, Delayed: true}
		}
	})
}

func (m *Model) startCreate() *Model {
	m.prevState = m.state
	m.state = stateTopicCreate
	tc := pages.NewTopicCreateModel(m.ctx, m.client)
	m.activePage = &tc
	return m
}

func (m *Model) startConsume(msg common.ConsumeTopicMsg) tea.Cmd {
	m.prevState = m.state
	m.state = stateTopicConsume
	c := pages.NewConsumerModel(m.ctx, m.client, msg.Topic, msg.Partition, msg.Seek, m.cfg.MaxMemoryMB)
	if msg.Value > 0 {
		c.SeekConfig.Value = msg.Value
	}
	m.activePage = &c
	m.activePage.HandleSize(m.width, m.height-4)
	return m.activePage.Init()
}

func (m *Model) startProduce(msg common.ProduceTopicMsg) tea.Cmd {
	m.prevState = m.state
	m.state = stateTopicProduce
	p := pages.NewTopicProduceModel(m.ctx, m.client, msg.Topic, msg.Partition)
	m.activePage = &p
	return m.activePage.Init()
}

func (m *Model) startConfig(msg common.EditConfigMsg) tea.Cmd {
	m.prevState = m.state
	m.state = stateTopicConfig
	tc := pages.NewTopicConfigModel(m.ctx, m.client, msg.Topic)
	m.activePage = &tc
	m.updateLayout()
	return m.activePage.Init()
}

func (m *Model) startResetOffset(msg common.ResetOffsetMsg) tea.Cmd {
	m.prevState = m.state
	m.state = stateResetOffset
	ro := pages.NewResetOffsetModel(m.ctx, m.client, msg.GroupID, msg.GroupState, msg.Topic, msg.Partition, msg.Current)
	m.activePage = &ro
	return m.activePage.Init()
}

func (m *Model) delegateUpdate(msg tea.Msg, cmds *[]tea.Cmd) {
	if m.state != stateMain && m.activePage != nil {
		*cmds = append(*cmds, m.activePage.Update(msg))
		return
	}

	km, isKey := msg.(tea.KeyMsg)
	
	if m.mode == modeTopics {
		if !isKey || m.focus == focusSidebar {
			*cmds = append(*cmds, m.topics.Update(msg))
		}
		if m.topicDetail != nil && m.topics.ConfirmDelete == "" {
			if !isKey || m.focus == focusDetails || strings.Contains("cbpre", km.String()) {
				*cmds = append(*cmds, m.topicDetail.Update(msg))
			}
		}
	} else {
		if !isKey || m.focus == focusSidebar {
			*cmds = append(*cmds, m.groups.Update(msg))
		}
		if m.groupDetail != nil {
			if !isKey || m.focus == focusDetails || strings.Contains("cbr", km.String()) {
				*cmds = append(*cmds, m.groupDetail.Update(msg))
			}
		}
	}
}

func (m *Model) updateLayout() {
	if m.width <= 2 || m.height <= 5 {
		return
	}

	sw := m.width * 40 / 100
	swInner := sw - 2
	if swInner < 0 { swInner = 0 }
	
	hInner := m.height - 5
	if hInner < 0 { hInner = 0 }
	
	m.topics.HandleSize(swInner, hInner)
	m.groups.HandleSize(swInner, hInner)
	
	dw := m.width - sw - 2
	dwInner := dw - 2
	if dwInner < 0 { dwInner = 0 }

	if m.topicDetail != nil {
		m.topicDetail.HandleSize(dwInner, hInner)
	}
	if m.groupDetail != nil {
		m.groupDetail.HandleSize(dwInner, hInner)
	}
	if m.activePage != nil {
		activeW := m.width - 2
		if activeW < 0 { activeW = 0 }
		m.activePage.HandleSize(activeW, hInner)
	}
}

func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	brokerLabel := m.client.GetBroker()
	if !m.client.IsWriteEnabled() {
		brokerLabel += " [READ-ONLY]"
	}
	
	headerText := " KAF - simple kafka client [" + brokerLabel + "]"
	if m.client.IsProduction() {
		headerText = " [ PRODUCTION ] " + headerText
	}
	header := components.TitleStyle.Width(m.width).Render(headerText)

	var content string
	if m.state != stateMain && m.activePage != nil {
		if _, ok := m.activePage.(*pages.ConsumerModel); ok {
			content = m.activePage.View()
		} else {
			pageStyle := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(components.GlobalTheme.PrimaryColor).
				Width(m.width - 2).
				Height(m.height - 5)
				
			content = header + "\n" + pageStyle.Render(m.activePage.View())
		}
	} else {
		sw := m.width * 40 / 100
		dw := m.width - sw
		
		sideStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Width(sw - 2).
			Height(m.height - 5)

		detailStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Width(dw - 2).
			Height(m.height - 5)
		
		if m.focus == focusSidebar {
			sideStyle = sideStyle.BorderForeground(components.GlobalTheme.PrimaryColor)
			detailStyle = detailStyle.BorderForeground(components.GlobalTheme.DimColor)
		} else {
			sideStyle = sideStyle.BorderForeground(components.GlobalTheme.DimColor)
			detailStyle = detailStyle.BorderForeground(components.GlobalTheme.PrimaryColor)
		}

		var sidebar, detail string
		help := components.GlobalTheme.HelpStyle.Render(" g:switch mode • tab:focus • C:context • n:new • d:delete • c:tail • b:history • p:produce • e:config")
		if m.mode == modeTopics {
			sidebar = m.topics.View()
			if m.topicDetail != nil {
				detail = m.topicDetail.View()
			}
		} else {
			sidebar = m.groups.View()
			if m.groupDetail != nil {
				detail = m.groupDetail.View()
			}
			help = components.GlobalTheme.HelpStyle.Render(" g:switch mode • tab:focus • C:context • /:filter • r:reset")
		}

		if m.focus == focusDetails {
			if m.mode == modeTopics {
				help = components.GlobalTheme.HelpStyle.Render(" up/down: select • c:tail part • b:history part • p:produce part • e:config • esc:back")
			} else {
				help = components.GlobalTheme.HelpStyle.Render(" up/down: select • c:tail part • b:history part • r:reset • esc:back")
			}
		}

		content = header + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, 
			sideStyle.Render(sidebar), 
			detailStyle.Render(detail)) + "\n" + help
	}

	if m.toastMsg != "" {
		style := components.GlobalTheme.InfoStyle
		prefix := " INFO: "
		if m.toastIsError {
			style = components.GlobalTheme.ToastStyle
			prefix = " ERROR: "
		}
		
		toast := style.Width(m.width).Render(prefix + m.toastMsg)
		
		// If we are in stateMain, we can replace the 'help' line which is the last line
		// Otherwise, we append it but we've already reserved space if needed.
		// For pages like ConsumerModel, the footer is part of their View().
		lines := strings.Split(content, "\n")
		if len(lines) > 0 {
			lines[len(lines)-1] = toast
			return strings.Join(lines, "\n")
		}
		return toast
	}

	return content
}

func (m *Model) isFiltering() bool {
	if m.mode == modeTopics {
		return m.topics.List.FilterState() == 1 && m.topics.List.FilterValue() != ""
	}
	return m.groups.List.FilterState() == 1 && m.groups.List.FilterValue() != ""
}

func (m *Model) deleteTopic(topic string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.DeleteTopic(context.TODO(), topic); err != nil {
			return common.ErrMsg{Err: err}
		}
		return common.TopicDeletedMsg{Topic: topic}
	}
}
