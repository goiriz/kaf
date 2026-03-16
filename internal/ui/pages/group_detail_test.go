package pages

import (
	"context"
	"testing"

	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui/common"
)

func TestGroupDetailModel(t *testing.T) {
	mock := &MockCluster{
		GroupDetail: &kafka.GroupDetail{
			ID:    "group-1",
			State: "Stable",
			Offsets: []kafka.GroupOffset{
				{Topic: "topic-1", Partition: 0, GroupOffset: 10, LatestOffset: 15, Lag: 5},
			},
		},
	}

	m := NewGroupDetailModel(context.TODO(), mock, "group-1")
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Expected Init to return a command")
	}

	// Simulate receiving the group detail loaded message
	msg := common.GroupDetailLoadedMsg{Detail: mock.GroupDetail}
	_ = m.Update(msg)

	if m.Loading {
		t.Error("Expected Loading to be false after GroupDetailLoadedMsg")
	}

	if m.Detail.ID != "group-1" {
		t.Errorf("Expected group ID to be group-1, got %s", m.Detail.ID)
	}

	if len(m.Detail.Offsets) != 1 {
		t.Errorf("Expected 1 offset, got %d", len(m.Detail.Offsets))
	}
}
