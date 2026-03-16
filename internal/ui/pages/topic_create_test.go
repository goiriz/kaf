package pages

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"
	"testing"
)

func TestTopicCreateModel(t *testing.T) {
	mock := &MockCluster{}

	m := NewTopicCreateModel(context.TODO(), mock)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Expected Init to return a command")
	}

	// Test basic initialization
	if len(m.Inputs) != 3 {
		t.Errorf("Expected 3 inputs, got %d", len(m.Inputs))
	}

	// Test navigation with Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_ = m.Update(msg)

}
