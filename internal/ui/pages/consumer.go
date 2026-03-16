package pages

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/goiriz/kaf/internal/config"
	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui/common"
	"github.com/goiriz/kaf/internal/ui/components"
)

type ConsumerModel struct {
	Client        kafka.Cluster
	Topic         string
	Partition     int
	SeekConfig    kafka.ConsumeConfig
	Messages      []kafka.Message
	Viewport      viewport.Model
	SearchInput   textinput.Model
	Searching     bool
	Ready         bool
	Ctx           context.Context
	Cancel        context.CancelFunc
	closeOnce     sync.Once
	wg            sync.WaitGroup
	MsgChan       chan []kafka.Message
	ErrChan       chan error
	Width, Height int

	// Selection & Navigation
	Cursor int
	Follow bool

	// Visibility
	ShowSensitive bool

	// Thread safety
	messagesMu sync.RWMutex

	// Performance & Security caps
	contentBuffer  strings.Builder
	totalBytes     int64
	maxMemoryBytes int64
}

func NewConsumerModel(ctx context.Context, client kafka.Cluster, topic string, partition int, seek kafka.SeekType, maxMemoryMB int) ConsumerModel {
	ctx, cancel := context.WithCancel(ctx)
	ti := textinput.New()
	ti.Placeholder = "Filter messages..."
	ti.Prompt = " / "

	cfg := kafka.ConsumeConfig{
		Topic:     topic,
		Partition: partition,
		Seek:      seek,
		Limit:     5000,
	}

	if maxMemoryMB <= 0 {
		maxMemoryMB = 100
	}

	return ConsumerModel{
		Client: client, Topic: topic, Partition: partition,
		SeekConfig: cfg, Follow: (seek == kafka.SeekEnd),
		Ctx: ctx, Cancel: cancel, MsgChan: make(chan []kafka.Message), ErrChan: make(chan error),
		SearchInput: ti,
		maxMemoryBytes: int64(maxMemoryMB) * 1024 * 1024,
	}
}

func (m *ConsumerModel) Init() tea.Cmd {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.Client.Consume(m.Ctx, m.SeekConfig, m.MsgChan, m.ErrChan)
		m.closeOnce.Do(func() { m.Cancel() })
	}()
	return m.waitForMessage()
}

func (m *ConsumerModel) Close() {
	m.closeOnce.Do(func() { m.Cancel() })
	go func() {
		m.wg.Wait()
		close(m.MsgChan)
		close(m.ErrChan)
	}()
}

func (m *ConsumerModel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	if m.Searching {
		if km, ok := msg.(tea.KeyMsg); ok && (km.String() == "enter" || km.String() == "esc") {
			m.Searching = false
			m.SearchInput.Blur()
			m.refresh()
			return nil
		}
		m.SearchInput, cmd = m.SearchInput.Update(msg)
		return cmd
	}

	switch msg := msg.(type) {
	case common.ErrMsg:
		return func() tea.Msg { return common.ErrMsg{Err: fmt.Errorf("consumer error: %w", msg.Err)} }
	case []kafka.Message:
		m.messagesMu.Lock()
		for _, mNew := range msg {
			m.Messages = append(m.Messages, mNew)
			m.totalBytes += int64(len(mNew.Key) + len(mNew.Value))
		}
		
		// Memory Protection: Purge oldest if exceeding configured limit
		purged := false
		for m.totalBytes > m.maxMemoryBytes && len(m.Messages) > 0 {
			oldest := m.Messages[0]
			m.totalBytes -= int64(len(oldest.Key) + len(oldest.Value))
			if oldest.FormattedValue != nil {
				m.totalBytes -= int64(len(*oldest.FormattedValue))
			}
			m.Messages = m.Messages[1:]
			if m.Cursor > 0 {
				m.Cursor--
			}
			purged = true
		}
		m.messagesMu.Unlock()

		if purged {
			m.refresh()
		} else {
			// Fast path: just append new ones if following
			filter := m.SearchInput.Value()
			appended := false
			m.messagesMu.RLock()
			for idx, message := range msg {
				if common.FilterMatches(string(message.Key), filter) || common.FilterMatches(string(message.Value), filter) {
					// Index in m.Messages is len(m.Messages)-len(msg)+idx
					// This is safe because we just appended msg to m.Messages while holding the Lock
					m.appendMessage(len(m.Messages)-len(msg)+idx, filter, false)
					appended = true
				}
			}
			m.messagesMu.RUnlock()
			
			if appended && m.Ready {
				m.Viewport.SetContent(m.contentBuffer.String())
				if m.Follow {
					m.Viewport.GotoBottom()
					m.Cursor = len(m.Messages) - 1
				}
			}
		}
		return m.waitForMessage()
	case tea.KeyMsg:
		switch msg.String() {
		case "/":
			m.Searching = true
			m.SearchInput.Focus()
			m.SearchInput.SetValue("")
			return nil
		case "up", "k":
			m.Follow = false
			if m.Cursor > 0 {
				m.Cursor--
				m.refresh()
			}
			return nil
		case "down", "j":
			if m.Cursor < len(m.Messages)-1 {
				m.Cursor++
				if m.Cursor == len(m.Messages)-1 {
					m.Follow = true
				}
				m.refresh()
			}
			return nil
		case "p":
			return func() tea.Msg { return common.ProduceTopicMsg{Topic: m.Topic, Partition: m.Partition} }
		case "y":
			if len(m.Messages) > 0 && m.Cursor >= 0 && m.Cursor < len(m.Messages) {
				target := m.Messages[m.Cursor]
				content := m.Client.GetDecoder().Decode(m.Topic, target.Value)
				
				status := "(redacted)"
				if !m.ShowSensitive {
					// Apply redaction if sensitive data is hidden in UI
					if redactKeys := m.Client.GetRedactKeys(); len(redactKeys) > 0 || len(config.DefaultRedactKeys) > 0 {
						content = common.RedactJSON(content, redactKeys)
					}
				} else {
					status = "(UNREDACTED)"
				}

				if err := clipboard.WriteAll(content); err != nil {
					return func() tea.Msg { return common.ErrMsg{Err: fmt.Errorf("Clipboard fail: %w", err)} }
				}

				// SAFETY: Auto-clear clipboard after 30 seconds to prevent lingering secrets
				go func(copied string) {
					time.Sleep(30 * time.Second)
					if current, err := clipboard.ReadAll(); err == nil && current == copied {
						clipboard.WriteAll("")
					}
				}(content)

				return func() tea.Msg { return common.InfoMsg{Text: fmt.Sprintf("Copied offset %d %s (auto-clears in 30s)", target.Offset, status)} }
			}
		case "u":
			m.ShowSensitive = !m.ShowSensitive
			// Invalidate cache to force re-formatting
			m.messagesMu.Lock()
			m.totalBytes = 0
			for i := range m.Messages {
				m.Messages[i].FormattedValue = nil
				m.totalBytes += int64(len(m.Messages[i].Key) + len(m.Messages[i].Value))
			}
			m.messagesMu.Unlock()
			m.refresh()
			m.Viewport.SetYOffset(m.Viewport.YOffset) // Force viewport sync
			status := "ENABLED"
			if m.ShowSensitive { status = "DISABLED (SENSITIVE DATA VISIBLE)" }
			return func() tea.Msg { return common.InfoMsg{Text: "Redaction " + status} }
		case "v":
			name := m.Client.CycleDecoder()
			// Invalidate cache to force re-formatting with the new decoder
			m.messagesMu.Lock()
			m.totalBytes = 0
			for i := range m.Messages {
				m.Messages[i].FormattedValue = nil
				m.totalBytes += int64(len(m.Messages[i].Key) + len(m.Messages[i].Value))
			}
			m.messagesMu.Unlock()
			m.refresh()
			m.Viewport.SetYOffset(0) // Reset scroll position to top of first message
			return func() tea.Msg { return common.InfoMsg{Text: "Decoder switched to: " + name} }
		}
	}
	m.Viewport, cmd = m.Viewport.Update(msg)
	return cmd
}

func (m *ConsumerModel) appendMessage(index int, filter string, selected bool) {
	msg := &m.Messages[index]
	style := components.GlobalTheme.SecondaryStyle
	if selected {
		style = components.GlobalTheme.SelectedStyle.Copy().Background(components.GlobalTheme.SecondaryColor).Foreground(lipgloss.Color("230"))
	}

	headerText := fmt.Sprintf("PART:%d OFF:%d TIME:%s", msg.Partition, msg.Offset, msg.Time.Local().Format("15:04:05"))
	if selected {
		headerText = "> " + headerText
	}
	m.contentBuffer.WriteString(style.Render(headerText) + "\n")
	
	if len(msg.Key) > 0 {
		m.contentBuffer.WriteString(components.GlobalTheme.SelectedStyle.Render("KEY: "+string(msg.Key)) + "\n")
	}

	// Render Headers if present
	if len(msg.Headers) > 0 {
		var hStr []string
		for k, v := range msg.Headers {
			hStr = append(hStr, fmt.Sprintf("%s=%s", k, string(v)))
		}
		m.contentBuffer.WriteString(components.GlobalTheme.InternalStyle.Bold(true).Render(" 🏷️ HEADERS: "+strings.Join(hStr, " | ")) + "\n")
	}

	var val string
	if msg.FormattedValue != nil {
		val = *msg.FormattedValue
	} else {
		redactKeys := m.Client.GetRedactKeys()
		
		if !m.ShowSensitive && config.ShouldRedact(string(msg.Key), redactKeys) {
			val = "[REDACTED] (Reason: sensitive Kafka Key)"
		} else {
			val = m.Client.GetDecoder().Decode(m.Topic, msg.Value)
			if !m.ShowSensitive && (len(redactKeys) > 0 || len(config.DefaultRedactKeys) > 0) {
				val = common.RedactJSON(val, redactKeys)
			}
		}
		msg.FormattedValue = &val
		m.totalBytes += int64(len(val))
	}

	if filter != "" && !strings.Contains(val, "\n") {
		f := strings.ToLower(filter)
		val = strings.ReplaceAll(val, f, components.GlobalTheme.HighlightStyle.Render(f))
	}

	sepWidth := m.Width - 4
	if sepWidth < 0 { sepWidth = 0 }
	
	sepStyle := components.GlobalTheme.DimStyle
	if selected {
		sepStyle = components.GlobalTheme.SelectedStyle
	}
	
	m.contentBuffer.WriteString(val + "\n" + sepStyle.Render(strings.Repeat("─", sepWidth)) + "\n\n")
}

func (m *ConsumerModel) refresh() {
	if !m.Ready {
		return
	}

	m.messagesMu.RLock()
	defer m.messagesMu.RUnlock()

	filter := m.SearchInput.Value()
	
	// Reuse indices slice to avoid allocations if possible
	var filteredIndices []int
	if filter == "" {
		// Optimization: no filter, use all messages
		filteredIndices = make([]int, len(m.Messages))
		for i := range m.Messages {
			filteredIndices[i] = i
		}
	} else {
		for i, msg := range m.Messages {
			if common.FilterMatches(string(msg.Key), filter) || common.FilterMatches(string(msg.Value), filter) {
				filteredIndices = append(filteredIndices, i)
			}
		}
	}

	if len(filteredIndices) == 0 {
		m.Viewport.SetContent(components.GlobalTheme.DimStyle.Render(" No messages matching filter."))
		return
	}

	// UI Virtualization: Find current cursor in filtered list
	viewIdx := -1
	for i, origIdx := range filteredIndices {
		if origIdx == m.Cursor {
			viewIdx = i
			break
		}
	}
	// Fallback if cursor message is filtered out: find nearest
	if viewIdx == -1 {
		viewIdx = 0 
	}

	// Window: 20 messages before, 50 after cursor
	start := viewIdx - 20
	if start < 0 { start = 0 }
	end := viewIdx + 50
	if end > len(filteredIndices) { end = len(filteredIndices) }

	// Reset buffer once and grow it to a reasonable size to avoid reallocs
	m.contentBuffer.Reset()
	m.contentBuffer.Grow(4096 * (end - start))

	selectedLine := 0
	for i := start; i < end; i++ {
		if i == viewIdx {
			selectedLine = strings.Count(m.contentBuffer.String(), "\n")
		}
		origIdx := filteredIndices[i]
		m.appendMessage(origIdx, filter, origIdx == m.Cursor)
	}

	m.Viewport.SetContent(m.contentBuffer.String())
	
	// Ensure selected message is visible
	if !m.Follow {
		currY := m.Viewport.YOffset
		height := m.Viewport.Height
		
		// If cursor is below view, scroll down
		if selectedLine >= currY+height-2 {
			m.Viewport.SetYOffset(selectedLine - height + 5)
		}
		// If cursor is above view, scroll up
		if selectedLine < currY {
			m.Viewport.SetYOffset(selectedLine)
		}
	} else {
		m.Viewport.GotoBottom()
	}
}

func (m *ConsumerModel) HandleSize(w, h int) {
	m.Width, m.Height = w, h
	headerH, footerH := 1, 1
	if !m.Ready {
		m.Viewport = viewport.New(w, h-headerH-footerH)
		m.Ready = true
		m.refresh()
	} else {
		m.Viewport.Width = w
		m.Viewport.Height = h - headerH - footerH
	}
}


func (m *ConsumerModel) View() string {
	if !m.Ready {
		return "Initializing..."
	}
	var s strings.Builder
	header := components.GlobalTheme.TitleStyle.Width(m.Width).Render(fmt.Sprintf(" CONSUMING: %s (P:%d)", m.Topic, m.Partition))
	s.WriteString(header + "\n")
	s.WriteString(m.Viewport.View() + "\n")
	
	redactLabel := "Unredact"
	if m.ShowSensitive {
		redactLabel = "REDACT"
	}

	footerText := fmt.Sprintf(" [ESC] Back | [/] Filter | [UP/DOWN] Select | [Y] Copy | [V] Dec:%s | [U] %s", m.Client.GetDecoder().Name(), redactLabel)
	if m.Searching {
		footerText = m.SearchInput.View()
	}
	footer := components.GlobalTheme.SecondaryStyle.Width(m.Width).Render(footerText)
	s.WriteString(footer)
	
	return s.String()
}

func (m *ConsumerModel) waitForMessage() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-m.MsgChan:
			if !ok { return nil }
			return msg
		case err, ok := <-m.ErrChan:
			if !ok || err == nil { return nil }
			return common.ErrMsg{Err: err}
		case <-m.Ctx.Done():
			return nil
		}
	}
}
