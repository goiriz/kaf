package kafka

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"testing"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/goiriz/kaf/internal/config"
)

type mockKafkaAPI struct {
	metadataResp    *kafkago.MetadataResponse
	listGroupsResp  *kafkago.ListGroupsResponse
	err             error
}

func (m *mockKafkaAPI) Metadata(ctx context.Context, req *kafkago.MetadataRequest) (*kafkago.MetadataResponse, error) {
	return m.metadataResp, m.err
}

func (m *mockKafkaAPI) CreateTopics(ctx context.Context, req *kafkago.CreateTopicsRequest) (*kafkago.CreateTopicsResponse, error) {
	res := &kafkago.CreateTopicsResponse{Errors: make(map[string]error)}
	for _, t := range req.Topics {
		res.Errors[t.Topic] = m.err
	}
	return res, nil
}

func (m *mockKafkaAPI) DeleteTopics(ctx context.Context, req *kafkago.DeleteTopicsRequest) (*kafkago.DeleteTopicsResponse, error) {
	res := &kafkago.DeleteTopicsResponse{Errors: make(map[string]error)}
	for _, t := range req.Topics {
		res.Errors[t] = m.err
	}
	return res, nil
}

func (m *mockKafkaAPI) ListOffsets(ctx context.Context, req *kafkago.ListOffsetsRequest) (*kafkago.ListOffsetsResponse, error) {
	return &kafkago.ListOffsetsResponse{}, nil
}

func (m *mockKafkaAPI) ListGroups(ctx context.Context, req *kafkago.ListGroupsRequest) (*kafkago.ListGroupsResponse, error) {
	return m.listGroupsResp, m.err
}

func (m *mockKafkaAPI) DescribeGroups(ctx context.Context, req *kafkago.DescribeGroupsRequest) (*kafkago.DescribeGroupsResponse, error) { return nil, nil }
func (m *mockKafkaAPI) OffsetFetch(ctx context.Context, req *kafkago.OffsetFetchRequest) (*kafkago.OffsetFetchResponse, error) { return nil, nil }
func (m *mockKafkaAPI) OffsetCommit(ctx context.Context, req *kafkago.OffsetCommitRequest) (*kafkago.OffsetCommitResponse, error) { return nil, nil }
func (m *mockKafkaAPI) DescribeConfigs(ctx context.Context, req *kafkago.DescribeConfigsRequest) (*kafkago.DescribeConfigsResponse, error) { return nil, nil }
func (m *mockKafkaAPI) IncrementalAlterConfigs(ctx context.Context, req *kafkago.IncrementalAlterConfigsRequest) (*kafkago.IncrementalAlterConfigsResponse, error) { return nil, nil }

func TestClient_ListTopics(t *testing.T) {
	mock := &mockKafkaAPI{
		metadataResp: &kafkago.MetadataResponse{
			Topics: []kafkago.Topic{
				{Name: "topic-a", Partitions: []kafkago.Partition{{ID: 0}}},
			},
		},
	}

	client := &Client{Brokers: []string{"localhost:9092"}, api: mock}
	topics, err := client.ListTopics(context.Background())
	if err != nil || len(topics) != 1 {
		t.Fatalf("Expected 1 topic, got %d (err: %v)", len(topics), err)
	}
}

func TestClient_CreateTopic(t *testing.T) {
	mock := &mockKafkaAPI{}
	
	t.Run("ReadOnly", func(t *testing.T) {
		client := NewClient(&config.Context{Brokers: []string{"localhost:9092"}, WriteEnabled: false})
		client.api = mock
		err := client.CreateTopic(context.Background(), "new-topic", 3, 1)
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("Expected ErrReadOnly, got %v", err)
		}
	})

	t.Run("WriteEnabled", func(t *testing.T) {
		client := NewClient(&config.Context{Brokers: []string{"localhost:9092"}, WriteEnabled: true})
		client.api = mock
		err := client.CreateTopic(context.Background(), "new-topic", 3, 1)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

func TestClient_DeleteTopic(t *testing.T) {
	mock := &mockKafkaAPI{}

	t.Run("ReadOnly", func(t *testing.T) {
		client := NewClient(&config.Context{Brokers: []string{"localhost:9092"}, WriteEnabled: false})
		client.api = mock
		err := client.DeleteTopic(context.Background(), "old-topic")
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("Expected ErrReadOnly, got %v", err)
		}
	})

	t.Run("WriteEnabled", func(t *testing.T) {
		client := NewClient(&config.Context{Brokers: []string{"localhost:9092"}, WriteEnabled: true})
		client.api = mock
		err := client.DeleteTopic(context.Background(), "old-topic")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

func TestClient_GetTopicDetail(t *testing.T) {
	mock := &mockKafkaAPI{
		metadataResp: &kafkago.MetadataResponse{
			Topics: []kafkago.Topic{
				{Name: "topic-a", Partitions: []kafkago.Partition{{ID: 0, Leader: kafkago.Broker{Host: "h", Port: 9092}}}},
			},
		},
	}
	client := NewClient(&config.Context{Brokers: []string{"localhost:9092"}})
	client.api = mock
	detail, err := client.GetTopicDetail(context.Background(), "topic-a")
	if err != nil || detail.Name != "topic-a" {
		t.Errorf("Expected topic-a detail, got %v (err: %v)", detail, err)
	}
}

func TestClient_ListGroups(t *testing.T) {
	mock := &mockKafkaAPI{
		listGroupsResp: &kafkago.ListGroupsResponse{
			Groups: []kafkago.ListGroupsResponseGroup{{GroupID: "group-1", ProtocolType: "consumer"}},
		},
	}
	client := NewClient(&config.Context{Brokers: []string{"localhost:9092"}})
	client.api = mock
	groups, err := client.ListGroups(context.Background())
	if err != nil || len(groups) != 1 {
		t.Errorf("Expected 1 group, got %v (err: %v)", groups, err)
	}
}

func TestClient_GetBroker(t *testing.T) {
	client := &Client{Brokers: []string{"test-broker:9092"}}
	if client.GetBroker() != "test-broker:9092" {
		t.Errorf("Expected test-broker:9092, got %s", client.GetBroker())
	}
}

func TestMapError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected error
	}{
		{"TopicNotFound", kafkago.UnknownTopicOrPartition, ErrTopicNotFound},
		{"ConnectionRefused", &net.OpError{Op: "dial", Err: &os.PathError{Err: errors.New("connection refused")}}, ErrConnectionRefused},
		{"Timeout", context.DeadlineExceeded, ErrTimeout},
		{"AuthFail", errors.New("SASL authentication failed"), ErrAuthenticationFail},
		{"Generic", errors.New("some other error"), errors.New("some other error")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapError(tt.input)
			if tt.expected.Error() != got.Error() {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestExecCommand(t *testing.T) {
	out, err := execCommand("echo hello-world")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if strings.TrimSpace(out) != "hello-world" {
		t.Errorf("expected hello-world, got %s", out)
	}

	_, err = execCommand("non-existent-command")
	if err == nil {
		t.Error("expected error for non-existent command, got nil")
	}
}

func TestConsume_RetryLogic(t *testing.T) {
	t.Run("ShouldRedact", func(t *testing.T) {
		redactKeys := []string{"api_key", "secret"}
		if !config.ShouldRedact("api_key", redactKeys) {
			t.Error("expected api_key to be redacted")
		}
		if !config.ShouldRedact("my_password", nil) { // Default keys
			t.Error("expected password to be redacted by default")
		}
		if config.ShouldRedact("normal_field", redactKeys) {
			t.Error("did not expect normal_field to be redacted")
		}
	})
}
