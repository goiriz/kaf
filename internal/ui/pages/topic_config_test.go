package pages

import (
	"context"
	"testing"

	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui/common"
)

func TestTopicConfigModel(t *testing.T) {
	mock := &MockCluster{
		ConfigEntries: []kafka.ConfigEntry{
			{Name: "retention.ms", Value: "604800000", IsDefault: true, ReadOnly: false},
			{Name: "cleanup.policy", Value: "delete", IsDefault: true, ReadOnly: false},
		},
	}

	m := NewTopicConfigModel(context.TODO(), mock, "topic-1")
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Expected Init to return a command")
	}

	// Simulate receiving the topic config loaded message
	msg := common.TopicConfigLoadedMsg{Entries: mock.ConfigEntries}
	_ = m.Update(msg)

	if m.Loading {
		t.Error("Expected Loading to be false after TopicConfigLoadedMsg")
	}

	items := m.List.Items()
	if len(items) != 2 {
		t.Errorf("Expected 2 items in list, got %d", len(items))
	}

	firstItem := items[0].(ConfigItem)
	if firstItem.Entry.Name != "retention.ms" {
		t.Errorf("Expected first item to be retention.ms, got %s", firstItem.Entry.Name)
	}
}
