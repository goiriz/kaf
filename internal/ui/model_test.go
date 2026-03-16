package ui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/goiriz/kaf/internal/config"
	"github.com/goiriz/kaf/internal/kafka"
)

// Full Mock for UI Testing
type MockCluster struct {
	Topics []kafkago.Topic
	Groups []kafkago.ListGroupsResponseGroup
	Broker string
}

func (m *MockCluster) GetBroker() string { return m.Broker }
func (m *MockCluster) ListTopics(ctx context.Context) ([]kafkago.Topic, error) { return m.Topics, nil }
func (m *MockCluster) CreateTopic(ctx context.Context, name string, p, r int) error { return nil }
func (m *MockCluster) DeleteTopic(ctx context.Context, name string) error { return nil }
func (m *MockCluster) GetTopicDetail(ctx context.Context, name string) (*kafka.TopicDetail, error) { return nil, nil }
func (m *MockCluster) Consume(ctx context.Context, cfg kafka.ConsumeConfig, mc chan<- []kafka.Message, ec chan<- error) {}
func (m *MockCluster) Produce(ctx context.Context, t string, p int, k, v []byte, h map[string]string) error { return nil }
func (m *MockCluster) ListGroups(ctx context.Context) ([]kafkago.ListGroupsResponseGroup, error) { return m.Groups, nil }
func (m *MockCluster) GetGroupDetail(ctx context.Context, id string) (*kafka.GroupDetail, error) { return nil, nil }
func (m *MockCluster) CommitOffset(ctx context.Context, groupID, topic string, partition int, offset int64) error { return nil }
func (m *MockCluster) GetTopicConfig(ctx context.Context, n string) ([]kafka.ConfigEntry, error) { return nil, nil }
func (m *MockCluster) UpdateTopicConfig(ctx context.Context, t, k, v string) error { return nil }
type mockDecoder struct{ kafka.DefaultDecoder }
func (d *mockDecoder) Errors() []string { return nil }
func (d *mockDecoder) Name() string   { return "MOCK" }

func (m *MockCluster) SetDecoder(d kafka.Decoder) {}
func (m *MockCluster) GetDecoder() kafka.Decoder { return &mockDecoder{} }
func (m *MockCluster) CycleDecoder() string { return "MOCK" }
func (m *MockCluster) IsWriteEnabled() bool { return false }
func (m *MockCluster) IsProduction() bool   { return false }
func (m *MockCluster) GetRedactKeys() []string { return nil }
func (m *MockCluster) Close()               {}

func TestModelNavigation(t *testing.T) {
	mock := &MockCluster{
		Broker: "localhost:9092",
		Topics: []kafkago.Topic{{Name: "test-topic", Partitions: make([]kafkago.Partition, 1)}},
	}

	m := NewModel(&config.Config{}, mock)

	// Test Initial State
	if m.mode != modeTopics {
		t.Errorf("Expected initial modeTopics, got %v", m.mode)
	}

	// Test Mode Toggle (g key)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")}
	m.Update(msg)

	if m.mode != modeGroups {
		t.Errorf("Expected modeGroups after pressing 'g', got %v", m.mode)
	}

	// Test Back to Topics
	m.Update(msg)
	if m.mode != modeTopics {
		t.Errorf("Expected modeTopics after pressing 'g' again, got %v", m.mode)
	}
}

func TestFocusTransition(t *testing.T) {
	mock := &MockCluster{Broker: "localhost:9092"}
	m := NewModel(&config.Config{}, mock)

	// Initial focus should be Sidebar
	if m.focus != focusSidebar {
		t.Errorf("Expected focusSidebar, got %v", m.focus)
	}

	// Press Right to focus Details
	msg := tea.KeyMsg{Type: tea.KeyRight}
	m.Update(msg)

	if m.focus != focusDetails {
		t.Errorf("Expected focusDetails after Right arrow, got %v", m.focus)
	}

	// Press Left to focus Sidebar
	msg = tea.KeyMsg{Type: tea.KeyLeft}
	m.Update(msg)

	if m.focus != focusSidebar {
		t.Errorf("Expected focusSidebar after Left arrow, got %v", m.focus)
	}
}

func TestModelView(t *testing.T) {
	mock := &MockCluster{Broker: "test-broker:9092"}
	m := NewModel(&config.Config{}, mock)
	m.width = 100
	m.height = 40

	view := m.View()
	if view == "" {
		t.Fatal("View returned empty string")
	}

	// Check if broker address is in the header
	if !contains(view, "test-broker:9092") {
		t.Errorf("Expected view to contain broker address, got: %s", view)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}())
}
