package pages

import (
	"context"
	"testing"

	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui/common"
)

func TestTopicDetailModel(t *testing.T) {
	mock := &MockCluster{
		TopicDetail: &kafka.TopicDetail{
			Name: "topic-1",
			Partitions: []kafka.PartitionDetail{
				{ID: 0, StartOffset: 0, EndOffset: 100},
				{ID: 1, StartOffset: 0, EndOffset: 200},
			},
		},
	}

	m := NewTopicDetailModel(context.TODO(), mock, "topic-1")
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Expected Init to return a command")
	}

	// Simulate receiving the topic detail loaded message
	msg := common.TopicDetailLoadedMsg{Detail: mock.TopicDetail}
	_ = m.Update(msg)

	if m.Loading {
		t.Error("Expected Loading to be false after TopicDetailLoadedMsg")
	}

	if m.Detail.Name != "topic-1" {
		t.Errorf("Expected topic name to be topic-1, got %s", m.Detail.Name)
	}

	if len(m.Detail.Partitions) != 2 {
		t.Errorf("Expected 2 partitions, got %d", len(m.Detail.Partitions))
	}
}
