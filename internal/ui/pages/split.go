package pages

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/goiriz/kaf/internal/ui/components"
)

type SplitPage struct {
	Left          tea.Model
	Right         tea.Model
	Width, Height int
	FocusLeft     bool
}

func (m *SplitPage) Init() tea.Cmd {
	return nil
}

func (m *SplitPage) Update(msg tea.Msg) tea.Cmd {
	// Delegation logic could go here if needed
	return nil
}

func (m *SplitPage) HandleSize(w, h int) {
	m.Width, m.Height = w, h
}

func (m *SplitPage) Close() {}

func (m *SplitPage) View() string {
	if m.Width <= 2 || m.Height <= 2 {
		return "Terminal too small"
	}

	sw := m.Width * 40 / 100
	dw := m.Width - sw

	swInner := sw - 2
	if swInner < 0 {
		swInner = 0
	}
	dwInner := dw - 2
	if dwInner < 0 {
		dwInner = 0
	}
	hInner := m.Height - 1
	if hInner < 0 {
		hInner = 0
	}

	sideStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(swInner).
		Height(hInner)

	detailStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(dwInner).
		Height(hInner)

	if m.FocusLeft {
		sideStyle = sideStyle.BorderForeground(components.GlobalTheme.PrimaryColor)
		detailStyle = detailStyle.BorderForeground(components.GlobalTheme.DimColor)
	} else {
		sideStyle = sideStyle.BorderForeground(components.GlobalTheme.DimColor)
		detailStyle = detailStyle.BorderForeground(components.GlobalTheme.PrimaryColor)
	}

	leftView := ""
	if m.Left != nil {
		leftView = m.Left.View()
	}
	rightView := ""
	if m.Right != nil {
		rightView = m.Right.View()
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, 
		sideStyle.Render(leftView), 
		detailStyle.Render(rightView))
}
