package pages

import (
	"context"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/goiriz/kaf/internal/kafka"
)

type MockCluster struct {
	kafka.Cluster
	Topics          []kafkago.Topic
	Groups          []kafkago.ListGroupsResponseGroup
	TopicDetail     *kafka.TopicDetail
	GroupDetail     *kafka.GroupDetail
	ConfigEntries   []kafka.ConfigEntry
	ProduceErr      error
	CreateTopicErr  error
	UpdateConfigErr error
	writeEnabled    bool
	isProduction    bool
}

func (m *MockCluster) GetBroker() string {
	return "localhost:9092"
}

func (m *MockCluster) ListTopics(ctx context.Context) ([]kafkago.Topic, error) {
	return m.Topics, nil
}

func (m *MockCluster) ListGroups(ctx context.Context) ([]kafkago.ListGroupsResponseGroup, error) {
	return m.Groups, nil
}

func (m *MockCluster) GetTopicDetail(ctx context.Context, name string) (*kafka.TopicDetail, error) {
	return m.TopicDetail, nil
}

func (m *MockCluster) GetGroupDetail(ctx context.Context, groupID string) (*kafka.GroupDetail, error) {
	return m.GroupDetail, nil
}

func (m *MockCluster) CommitOffset(ctx context.Context, groupID, topic string, partition int, offset int64) error {
	return nil
}

func (m *MockCluster) GetTopicConfig(ctx context.Context, name string) ([]kafka.ConfigEntry, error) {
	return m.ConfigEntries, nil
}

func (m *MockCluster) CreateTopic(ctx context.Context, name string, partitions, replication int) error {
	return m.CreateTopicErr
}

func (m *MockCluster) Produce(ctx context.Context, topic string, partition int, key, value []byte, headers map[string]string) error {
	return m.ProduceErr
}

func (m *MockCluster) UpdateTopicConfig(ctx context.Context, topic, key, value string) error {
	return m.UpdateConfigErr
}

type mockDecoder struct{ kafka.DefaultDecoder }
func (d *mockDecoder) Errors() []string { return nil }
func (d *mockDecoder) Name() string   { return "MOCK" }

func (m *MockCluster) SetDecoder(d kafka.Decoder) {}
func (m *MockCluster) GetDecoder() kafka.Decoder { return &mockDecoder{} }
func (m *MockCluster) CycleDecoder() string { return "MOCK" }
func (m *MockCluster) IsWriteEnabled() bool { return m.writeEnabled }
func (m *MockCluster) IsProduction() bool   { return m.isProduction }
func (m *MockCluster) GetRedactKeys() []string { return nil }
func (m *MockCluster) Close()               {}

func (m *MockCluster) Consume(ctx context.Context, cfg kafka.ConsumeConfig, messages chan<- []kafka.Message, errors chan<- error) {
	// Simple mock consume: just close or send nothing
}
