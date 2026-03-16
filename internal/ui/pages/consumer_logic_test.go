package pages

import (
	"context"
	"testing"
	"time"

	"github.com/goiriz/kaf/internal/kafka"
)

func TestConsumerMemoryCapping(t *testing.T) {
	mock := &MockCluster{}
	// Set a small limit: 500 bytes (to account for formatting overhead)
	m := NewConsumerModel(context.TODO(), mock, "topic", -1, kafka.SeekEnd, 0)
	m.maxMemoryBytes = 500 
	m.Ready = true // allow formatting

	// Send two large messages
	msg1 := kafka.Message{Value: make([]byte, 200), Time: time.Now()}
	msg2 := kafka.Message{Value: make([]byte, 200), Time: time.Now()}
	
	m.Update([]kafka.Message{msg1})
	// totalBytes = raw (200) + formatted (rendered during update if Ready)
	if m.totalBytes < 200 {
		t.Errorf("Expected at least 200 bytes, got %d", m.totalBytes)
	}

	m.Update([]kafka.Message{msg2})
	// Second message should trigger purge of msg1
	if len(m.Messages) != 1 {
		t.Errorf("Expected 1 message after purge, got %d", len(m.Messages))
	}
}

func TestConsumerKeyRedaction(t *testing.T) {
	mock := &MockCluster{}
	m := NewConsumerModel(context.TODO(), mock, "topic", -1, kafka.SeekEnd, 100)
	
	// Message with sensitive key
	msg := kafka.Message{
		Key: []byte("password"),
		Value: []byte("toto"),
		Time: time.Now(),
	}
	
	m.Messages = append(m.Messages, msg)
	m.appendMessage(0, "", false) // Render index 0
	
	if m.Messages[0].FormattedValue == nil {
		t.Fatal("FormattedValue should have been cached")
	}
	
	val := *m.Messages[0].FormattedValue
	if !contains(val, "[REDACTED]") {
		t.Errorf("Value should be redacted when key is 'password'. Got: %s", val)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
