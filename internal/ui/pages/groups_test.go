package pages

import (
	"context"
	"testing"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/goiriz/kaf/internal/ui/common"
)

func TestGroupsModel(t *testing.T) {
	mock := &MockCluster{
		Groups: []kafkago.ListGroupsResponseGroup{
			{GroupID: "group-1", ProtocolType: "consumer"},
		},
	}

	m := NewGroupsModel(context.TODO(), mock)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Expected Init to return a command")
	}

	// Simulate receiving the groups loaded message
	res := make([]common.GroupWithLag, len(mock.Groups))
	for i, g := range mock.Groups {
		res[i] = common.GroupWithLag{Group: g, TotalLag: 0}
	}
	msg := common.GroupsLoadedMsg{Groups: res}
	_ = m.Update(msg)

	if m.Loading {
		t.Error("Expected Loading to be false after GroupsLoadedMsg")
	}

	items := m.List.Items()
	if len(items) != 1 {
		t.Errorf("Expected 1 item in list, got %d", len(items))
	}

	firstItem := items[0].(GroupListItem)
	if firstItem.Group.GroupID != "group-1" {
		t.Errorf("Expected first item to be group-1, got %s", firstItem.Group.GroupID)
	}
}
