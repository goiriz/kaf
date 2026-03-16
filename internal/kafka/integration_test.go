package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/goiriz/kaf/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
)

func TestKafkaIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Recover from testcontainers panic if docker is not available
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("Skipping integration test due to Docker panic: %v", r)
		}
	}()

	// Start Kafka container
	kafkaContainer, err := kafka.Run(ctx,
		"confluentinc/confluent-local:7.5.0",
		kafka.WithClusterID("test-cluster"),
	)
	if err != nil {
		t.Skipf("Skipping integration test due to Docker startup failure: %v", err)
		return
	}

	defer func() {
		if err := testcontainers.TerminateContainer(kafkaContainer); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}()

	brokers, err := kafkaContainer.Brokers(ctx)
	require.NoError(t, err)

	client := NewClient(&config.Context{Brokers: brokers, WriteEnabled: true})
	topicName := "integration-test-topic"

	err = client.CreateTopic(ctx, topicName, 3, 1)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	topics, err := client.ListTopics(ctx)
	require.NoError(t, err)
	
	found := false
	for _, topicInfo := range topics {
		if topicInfo.Name == topicName {
			found = true
			require.Equal(t, 3, len(topicInfo.Partitions))
			break
		}
	}
	require.True(t, found, "Topic should exist in the list")

	err = client.Produce(ctx, topicName, 0, []byte("key1"), []byte(`{"message": "hello"}`), nil)
	require.NoError(t, err)

	msgChan := make(chan []Message)
	errChan := make(chan error)
	consumeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cfg := ConsumeConfig{
		Topic:     topicName,
		Partition: 0,
		Seek:      SeekBeginning,
		Limit:     100,
	}
	go client.Consume(consumeCtx, cfg, msgChan, errChan)

	select {
	case batch := <-msgChan:
		require.NotEmpty(t, batch)
		msg := batch[0]
		require.Equal(t, "key1", string(msg.Key))
		require.Equal(t, `{"message": "hello"}`, string(msg.Value))
	case err := <-errChan:
		t.Fatalf("Consume failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message")
	}

	err = client.DeleteTopic(ctx, topicName)
	require.NoError(t, err)
}
