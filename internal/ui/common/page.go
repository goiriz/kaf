package common

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Page defines the interface for all UI components that act as a standalone page.
type Page interface {
	Init() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	View() string
	HandleSize(w, h int)
	Close()
}

// Global Context key or helper if needed could go here.
