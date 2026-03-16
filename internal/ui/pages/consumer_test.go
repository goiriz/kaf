package pages

import (
	"context"
	"testing"
	"time"

	"github.com/goiriz/kaf/internal/kafka"
)

func TestConsumerModel(t *testing.T) {
	mock := &MockCluster{}

	m := NewConsumerModel(context.TODO(), mock, "topic-1", 0, kafka.SeekEnd, 100)
	m.Width = 80
	// We don't want to start the goroutine in a basic unit test as it might block or cause race conditions
	// but we can test the model's reaction to messages.

	msg := []kafka.Message{{
		Partition: 0,
		Offset:    1,
		Key:       []byte("key"),
		Value:     []byte("value"),
		Time:      time.Now(),
	}}

	_ = m.Update(msg)

	if len(m.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(m.Messages))
	}

	if string(m.Messages[0].Key) != "key" {
		t.Errorf("Expected key to be 'key', got %s", string(m.Messages[0].Key))
	}
}
