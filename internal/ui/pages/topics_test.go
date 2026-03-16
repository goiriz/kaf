package pages

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/goiriz/kaf/internal/ui/common"
)

func TestTopicsModelFetch(t *testing.T) {
	mock := &MockCluster{
		Topics: []kafkago.Topic{
			{Name: "topic-1", Partitions: make([]kafkago.Partition, 3)},
			{Name: "topic-2", Partitions: make([]kafkago.Partition, 1)},
		},
	}

	m := NewTopicsModel(context.TODO(), mock)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Expected Init to return a command")
	}

	// Simulate receiving the topics loaded message
	msg := common.TopicsLoadedMsg{Topics: mock.Topics}
	_ = m.Update(msg)

	if m.Loading {
		t.Error("Expected Loading to be false after TopicsLoadedMsg")
	}

	items := m.List.Items()
	if len(items) != 2 {
		t.Errorf("Expected 2 items in list, got %d", len(items))
	}

	firstItem := items[0].(TopicListItem)
	if firstItem.Topic.Name != "topic-1" {
		t.Errorf("Expected first item to be topic-1, got %s", firstItem.Topic.Name)
	}
}

func TestTopicsModelKeys(t *testing.T) {
	mock := &MockCluster{
		Topics: []kafkago.Topic{{Name: "test-topic", Partitions: make([]kafkago.Partition, 1)}},
	}
	mock.writeEnabled = true // Enable write for testing keys
	m := NewTopicsModel(context.TODO(), mock)
	m.Loading = false
	m.Focused = true
	m.List.SetItems([]list.Item{TopicListItem{Topic: mock.Topics[0]}})

	// 1. Test 'n' (New Topic)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("Expected command for 'n' key, got nil")
	}
	res := cmd()
	if _, ok := res.(common.CreateTopicMsg); !ok {
		t.Errorf("Expected CreateTopicMsg, got %T", res)
	}

	// 2. Test 'd' (Delete Confirmation)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}
	m.Update(msg)
	if m.ConfirmDelete != "test-topic" {
		t.Errorf("Expected ConfirmDelete to be 'test-topic', got %s", m.ConfirmDelete)
	}

	// 3. Test 'esc' (Cancel Delete)
	msg = tea.KeyMsg{Type: tea.KeyEsc}
	m.Update(msg)
	if m.ConfirmDelete != "" {
		t.Error("Expected ConfirmDelete to be cleared after 'esc'")
	}

	// 4. Test 'c' (Consume)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}
	cmd = m.Update(msg)
	res = cmd()
	if cmsg, ok := res.(common.ConsumeTopicMsg); !ok || cmsg.Topic != "test-topic" {
		t.Errorf("Expected ConsumeTopicMsg for 'test-topic', got %+v", res)
	}

	// 5. Test Filtering Guard (keys should be ignored when filtering)
	m.List.SetFilterState(list.Filtering)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	cmd = m.Update(msg)
	if cmd != nil {
		res := cmd()
		if _, ok := res.(common.CreateTopicMsg); ok {
			t.Error("CreateTopicMsg should NOT be emitted when filtering")
		}
	}
}
