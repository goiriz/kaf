package common

import (
	"time"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/goiriz/kaf/internal/kafka"
)

// Global Message Types
type ErrMsg struct{ Err error }
type InfoMsg struct{ Text string }
type TopicChangedMsg struct {
	Topic   string
	Delayed bool
}
type GroupChangedMsg struct {
	GroupID string
	Delayed bool
}
type CreateTopicMsg struct{}
type DeleteConfirmedMsg struct{ Topic string }
type TopicDeletedMsg struct{ Topic string }
type TopicCreatedMsg struct{}
type RefreshTopicsMsg struct{}
type EditConfigMsg struct{ Topic string }
type BackMsg struct{}
type OffsetResetSuccessMsg struct{ GroupID string }
type ClearToastMsg struct{ Time time.Time }
type SwitchContextMsg struct{ ContextName string }

type ConsumeTopicMsg struct {
	Topic     string
	Partition int
	Seek      kafka.SeekType
	Value     int64
}

type ProduceTopicMsg struct {
	Topic     string
	Partition int
}

type ResetOffsetMsg struct {
	GroupID    string
	GroupState string
	Topic      string
	Partition  int
	Current    int64
}

// Data Messages
type TopicsLoadedMsg struct{ Topics []kafkago.Topic }
type GroupWithLag struct {
	Group    kafkago.ListGroupsResponseGroup
	TotalLag int64
}

type GroupsLoadedMsg struct {
	Groups []GroupWithLag
}
type TopicDetailLoadedMsg struct{ Detail *kafka.TopicDetail }
type GroupDetailLoadedMsg struct{ Detail *kafka.GroupDetail }
type TopicConfigLoadedMsg struct{ Entries []kafka.ConfigEntry }
