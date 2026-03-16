package pages

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTopicProduceModel(t *testing.T) {
	mock := &MockCluster{}
	m := NewTopicProduceModel(context.TODO(), mock, "topic-1", 0)

	// Test initial state
	if m.Focus != 0 {
		t.Errorf("Expected initial focus 0, got %d", m.Focus)
	}

	// Test navigation (3 inputs now: Key, Value, Headers)
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Focus != 1 {
		t.Errorf("Expected focus index 1 after Down, got %d", m.Focus)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Focus != 2 {
		t.Errorf("Expected focus index 2 after Down, got %d", m.Focus)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Focus != 0 {
		t.Errorf("Expected focus to wrap to 0 after Down, got %d", m.Focus)
	}
}
