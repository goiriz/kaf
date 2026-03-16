package kafka

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"github.com/goiriz/kaf/internal/config"
)

type SeekType int

const (
	SeekBeginning SeekType = iota
	SeekEnd
	SeekTimestamp
	SeekOffset
	SeekLastN
)

type ConsumeConfig struct {
	Topic     string
	Partition int
	Seek      SeekType
	Value     int64 // Offset or Unix Timestamp in ms
	Limit     int
}

// Cluster defines the contract for Kafka operations.
type Cluster interface {
	ListTopics(ctx context.Context) ([]kafkago.Topic, error)
	GetBroker() string
	CreateTopic(ctx context.Context, name string, partitions, replication int) error
	DeleteTopic(ctx context.Context, name string) error
	GetTopicDetail(ctx context.Context, name string) (*TopicDetail, error)
	Consume(ctx context.Context, cfg ConsumeConfig, messages chan<- []Message, errors chan<- error)
	Produce(ctx context.Context, topic string, partition int, key, value []byte, headers map[string]string) error
	ListGroups(ctx context.Context) ([]kafkago.ListGroupsResponseGroup, error)
	GetGroupDetail(ctx context.Context, groupID string) (*GroupDetail, error)
	CommitOffset(ctx context.Context, groupID, topic string, partition int, offset int64) error
	GetTopicConfig(ctx context.Context, name string) ([]ConfigEntry, error)
	UpdateTopicConfig(ctx context.Context, topic, key, value string) error
	SetDecoder(d Decoder)
	GetDecoder() Decoder
	CycleDecoder() string
	IsWriteEnabled() bool
	IsProduction() bool
	GetRedactKeys() []string
	Close()
}

// kafkaAPI defines the subset of kafka-go Client methods we use.
type kafkaAPI interface {
	Metadata(ctx context.Context, req *kafkago.MetadataRequest) (*kafkago.MetadataResponse, error)
	CreateTopics(ctx context.Context, req *kafkago.CreateTopicsRequest) (*kafkago.CreateTopicsResponse, error)
	DeleteTopics(ctx context.Context, req *kafkago.DeleteTopicsRequest) (*kafkago.DeleteTopicsResponse, error)
	ListOffsets(ctx context.Context, req *kafkago.ListOffsetsRequest) (*kafkago.ListOffsetsResponse, error)
	ListGroups(ctx context.Context, req *kafkago.ListGroupsRequest) (*kafkago.ListGroupsResponse, error)
	DescribeGroups(ctx context.Context, req *kafkago.DescribeGroupsRequest) (*kafkago.DescribeGroupsResponse, error)
	OffsetFetch(ctx context.Context, req *kafkago.OffsetFetchRequest) (*kafkago.OffsetFetchResponse, error)
	OffsetCommit(ctx context.Context, req *kafkago.OffsetCommitRequest) (*kafkago.OffsetCommitResponse, error)
	DescribeConfigs(ctx context.Context, req *kafkago.DescribeConfigsRequest) (*kafkago.DescribeConfigsResponse, error)
	IncrementalAlterConfigs(ctx context.Context, req *kafkago.IncrementalAlterConfigsRequest) (*kafkago.IncrementalAlterConfigsResponse, error)
}

const (
	DefaultTimeout      = 10 * time.Second
	MetadataTimeout     = 5 * time.Second
	DefaultBatchSize    = 500
	ConsumeBatchSize    = 100
	ConsumeMaxWait      = 500 * time.Millisecond
	ConsumeMaxBytes     = 10e6
	ProduceBatchTimeout = 50 * time.Millisecond
)

type Client struct {
	Brokers      []string
	api          kafkaAPI
	dialer       *kafkago.Dialer
	trans        *kafkago.Transport
	decoder      Decoder
	baseDecoder  Decoder
	writeEnabled bool
	isProduction bool
	redactKeys   []string

	writers   map[string]*kafkago.Writer
	writersMu sync.Mutex
}

func (c *Client) SetDecoder(d Decoder) {
	c.decoder = d
}

func (c *Client) GetDecoder() Decoder {
	return c.decoder
}

func (c *Client) CycleDecoder() string {
	curr := c.decoder.Name()
	
	switch curr {
	case "AUTO":
		// If base is also AUTO, skip to HEX
		if c.baseDecoder.Name() == "AUTO" {
			c.decoder = HexDecoder{}
		} else {
			c.decoder = c.baseDecoder
		}
	case "PROTO/AVRO":
		c.decoder = HexDecoder{}
	case "HEX":
		c.decoder = DefaultDecoder{}
	default:
		c.decoder = DefaultDecoder{}
	}
	
	return c.decoder.Name()
}

func (c *Client) IsWriteEnabled() bool {
	return c.writeEnabled
}

func (c *Client) IsProduction() bool {
	return c.isProduction
}

func (c *Client) GetRedactKeys() []string {
	return c.redactKeys
}

func (c *Client) GetBroker() string {
	if len(c.Brokers) == 0 {
		return ""
	}
	return strings.Join(c.Brokers, ",")
}

func execCommand(cmdLine string) (string, error) {
	parts := strings.Fields(cmdLine)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	out, err := cmd.Output()
	return string(out), err
}

func getSASLMechanism(cfg *config.Context) (sasl.Mechanism, error) {
	if cfg == nil || cfg.SASL == nil {
		mech := os.Getenv("KAFKA_SASL_MECH")
		user := os.Getenv("KAFKA_SASL_USER")
		pass := os.Getenv("KAFKA_SASL_PASS")

		if user == "" || pass == "" {
			return nil, nil
		}

		switch mech {
		case "SCRAM-SHA-256":
			return scram.Mechanism(scram.SHA256, user, pass)
		case "SCRAM-SHA-512":
			return scram.Mechanism(scram.SHA512, user, pass)
		default:
			return plain.Mechanism{Username: user, Password: pass}, nil
		}
	}

	password := cfg.SASL.Password
	if cfg.SASL.PasswordCmd != "" {
		out, err := execCommand(cfg.SASL.PasswordCmd)
		if err != nil {
			return nil, fmt.Errorf("password command failed: %w", err)
		}
		password = strings.TrimSpace(out)
	}

	switch cfg.SASL.Mechanism {
	case "SCRAM-SHA-256":
		return scram.Mechanism(scram.SHA256, cfg.SASL.Username, password)
	case "SCRAM-SHA-512":
		return scram.Mechanism(scram.SHA512, cfg.SASL.Username, password)
	default:
		return plain.Mechanism{Username: cfg.SASL.Username, Password: password}, nil
	}
}

func NewClient(cfg *config.Context) *Client {
	brokers := cfg.Brokers
	if len(brokers) == 0 {
		brokers = []string{"localhost:9092"}
	}

	dialer := &kafkago.Dialer{
		Timeout:   DefaultTimeout,
		DualStack: true,
	}

	trans := &kafkago.Transport{
		Dial:        dialer.DialFunc,
		IdleTimeout: DefaultTimeout,
	}

	if cfg.TLS || os.Getenv("KAFKA_TLS_ENABLE") == "true" {
		tlsConfig := &tls.Config{InsecureSkipVerify: os.Getenv("KAFKA_TLS_INSECURE") == "true"}
		dialer.TLS = tlsConfig
		trans.TLS = tlsConfig
	}

	if mech, err := getSASLMechanism(cfg); err == nil && mech != nil {
		dialer.SASLMechanism = mech
		trans.SASL = mech
	}

	var dec Decoder = DefaultDecoder{}
	if cfg.SchemaRegistryURL != "" || len(cfg.ProtoPaths) > 0 {
		srPass := cfg.SchemaRegistryPass
		if cfg.SchemaRegistryPassCmd != "" {
			if out, err := execCommand(cfg.SchemaRegistryPassCmd); err == nil {
				srPass = strings.TrimSpace(out)
			}
		}
		dec = NewSRDecoder(SRConfig{
			URL:      cfg.SchemaRegistryURL,
			User:     cfg.SchemaRegistryUser,
			Pass:     srPass,
			Insecure: cfg.SchemaRegistryInsecure,
		}, cfg.ProtoPaths, cfg.TopicProtoMappings)
	}

	return &Client{
		Brokers:      brokers,
		dialer:       dialer,
		trans:        trans,
		decoder:      dec,
		baseDecoder:  dec,
		writeEnabled: cfg.WriteEnabled,
		isProduction: cfg.IsProduction,
		redactKeys:   cfg.RedactKeys,
		writers:      make(map[string]*kafkago.Writer),
		api: &kafkago.Client{
			Addr:      kafkago.TCP(brokers...),
			Timeout:   DefaultTimeout,
			Transport: trans,
		},
	}
}

var _ Cluster = (*Client)(nil)

func (c *Client) ListTopics(ctx context.Context) ([]kafkago.Topic, error) {
	metaCtx, cancel := context.WithTimeout(ctx, MetadataTimeout)
	defer cancel()

	resp, err := c.api.Metadata(metaCtx, &kafkago.MetadataRequest{
		Topics: nil,
	})
	if err != nil {
		if metaCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("cluster too large? full metadata request timed out: %w", MapError(err))
		}
		return nil, fmt.Errorf("failed to list topics: %w", MapError(err))
	}
	return resp.Topics, nil
}

func (c *Client) CreateTopic(ctx context.Context, name string, partitions, replication int) error {
	if !c.writeEnabled {
		return ErrReadOnly
	}
	resp, err := c.api.CreateTopics(ctx, &kafkago.CreateTopicsRequest{
		Topics: []kafkago.TopicConfig{{Topic: name, NumPartitions: partitions, ReplicationFactor: replication}},
	})
	if err != nil {
		return fmt.Errorf("failed to create topic %s: %w", name, err)
	}
	return resp.Errors[name]
}

func (c *Client) DeleteTopic(ctx context.Context, name string) error {
	if !c.writeEnabled {
		return ErrReadOnly
	}
	resp, err := c.api.DeleteTopics(ctx, &kafkago.DeleteTopicsRequest{Topics: []string{name}})
	if err != nil {
		return fmt.Errorf("failed to delete topic %s: %w", name, err)
	}
	return resp.Errors[name]
}

type PartitionDetail struct {
	ID          int
	Leader      string
	StartOffset int64
	EndOffset   int64
}

type TopicDetail struct {
	Name       string
	Partitions []PartitionDetail
}

func (c *Client) GetTopicDetail(ctx context.Context, name string) (*TopicDetail, error) {
	resp, err := c.api.Metadata(ctx, &kafkago.MetadataRequest{Topics: []string{name}})
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata for topic %s: %w", name, err)
	}
	if len(resp.Topics) == 0 {
		return nil, fmt.Errorf("topic not found")
	}

	t := resp.Topics[0]
	detail := &TopicDetail{Name: t.Name}

	offsets := make(map[int][2]int64)
	
	for i := 0; i < len(t.Partitions); i += DefaultBatchSize {
		end := i + DefaultBatchSize
		if end > len(t.Partitions) {
			end = len(t.Partitions)
		}
		
		batchPartitions := t.Partitions[i:end]
		reqs := make([]kafkago.OffsetRequest, len(batchPartitions)*2)
		for j, p := range batchPartitions {
			reqs[j*2] = kafkago.OffsetRequest{Partition: p.ID, Timestamp: kafkago.FirstOffset}
			reqs[j*2+1] = kafkago.OffsetRequest{Partition: p.ID, Timestamp: kafkago.LastOffset}
		}

		offResp, err := c.api.ListOffsets(ctx, &kafkago.ListOffsetsRequest{
			Addr:   kafkago.TCP(c.Brokers[0]),
			Topics: map[string][]kafkago.OffsetRequest{name: reqs},
		})

		if err == nil {
			if po, ok := offResp.Topics[name]; ok {
				for _, o := range po {
					v := offsets[o.Partition]
					if o.FirstOffset != 0 || o.LastOffset != 0 {
						if o.FirstOffset != -1 {
							v[0] = o.FirstOffset
						}
						if o.LastOffset != -1 {
							v[1] = o.LastOffset
						}
						offsets[o.Partition] = v
					}
				}
			}
		}
	}

	for _, p := range t.Partitions {
		detail.Partitions = append(detail.Partitions, PartitionDetail{
			ID:          p.ID,
			Leader:      net.JoinHostPort(p.Leader.Host, strconv.Itoa(p.Leader.Port)),
			StartOffset: offsets[p.ID][0],
			EndOffset:   offsets[p.ID][1],
		})
	}
	return detail, nil
}

type Message struct {
	Partition      int
	Offset         int64
	Key            []byte
	Value          []byte
	Headers        map[string][]byte
	Time           time.Time
	FormattedValue *string
}

func (c *Client) Consume(ctx context.Context, cfg ConsumeConfig, messages chan<- []Message, errors chan<- error) {
	startOffset := kafkago.LastOffset
	if cfg.Seek == SeekBeginning || cfg.Seek == SeekLastN {
		startOffset = kafkago.FirstOffset
	}

	rConfig := kafkago.ReaderConfig{
		Brokers:     c.Brokers,
		Topic:       cfg.Topic,
		Dialer:      c.dialer,
		MaxWait:     ConsumeMaxWait,
		MaxBytes:    ConsumeMaxBytes,
		StartOffset: startOffset,
	}

	if cfg.Partition >= 0 {
		rConfig.Partition = cfg.Partition
	}

	r := kafkago.NewReader(rConfig)
	defer r.Close()

	// Robust Seek logic
	switch cfg.Seek {
	case SeekOffset:
		r.SetOffset(cfg.Value)
	case SeekTimestamp:
		if err := r.SetOffsetAt(ctx, time.Unix(0, cfg.Value*int64(time.Millisecond))); err != nil {
			errors <- fmt.Errorf("failed to seek to timestamp: %w", err)
			return
		}
	case SeekLastN:
		if cfg.Partition >= 0 {
			r.SetOffset(kafkago.LastOffset)
			end := r.Offset()
			start := end - int64(cfg.Limit)
			if start < 0 { start = 0 }
			r.SetOffset(start)
		}
	}

	batch := make([]Message, 0, ConsumeBatchSize)
	batchMu := sync.Mutex{}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	sendBatch := func() {
		batchMu.Lock()
		defer batchMu.Unlock()
		if len(batch) > 0 {
			batchCopy := make([]Message, len(batch))
			copy(batchCopy, batch)
			select {
			case messages <- batchCopy:
				batch = batch[:0]
			case <-ctx.Done():
			}
		}
	}

	go func() {
		for {
			select {
			case <-ticker.C:
				sendBatch()
			case <-ctx.Done():
				return
			}
		}
	}()

	count := 0
	retryBackoff := 1 * time.Second

	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			
			// Auto-Reconnect with Backoff
			errors <- fmt.Errorf("connection lost, retrying in %v... (%v)", retryBackoff, err)
			select {
			case <-time.After(retryBackoff):
				retryBackoff *= 2
				if retryBackoff > 30*time.Second {
					retryBackoff = 30 * time.Second
				}
				continue 
			case <-ctx.Done():
				return
			}
		}

		retryBackoff = 1 * time.Second
		
		headers := make(map[string][]byte)
		for _, h := range m.Headers {
			headers[h.Key] = h.Value
		}

		batchMu.Lock()
		batch = append(batch, Message{
			Partition: m.Partition,
			Offset:    m.Offset,
			Key:       m.Key,
			Value:     m.Value,
			Headers:   headers,
			Time:      m.Time,
		})
		full := len(batch) >= ConsumeBatchSize
		batchMu.Unlock()

		if full {
			sendBatch()
		}

		if cfg.Limit > 0 {
			count++
			if count >= cfg.Limit {
				sendBatch()
				return
			}
		}
	}
}

func (c *Client) Produce(ctx context.Context, topic string, partition int, key, value []byte, headers map[string]string) error {
	if !c.writeEnabled {
		return ErrReadOnly
	}
	c.writersMu.Lock()
	w, ok := c.writers[topic]
	if !ok {
		w = &kafkago.Writer{
			Addr:         kafkago.TCP(c.Brokers...),
			Topic:        topic,
			Balancer:     &kafkago.LeastBytes{},
			Transport:    c.trans,
			BatchTimeout: ProduceBatchTimeout,
			Async:        false,
		}
		c.writers[topic] = w
	}
	c.writersMu.Unlock()

	msg := kafkago.Message{Key: key, Value: value}
	if partition >= 0 {
		msg.Partition = partition
	}

	if len(headers) > 0 {
		msg.Headers = make([]kafkago.Header, 0, len(headers))
		for k, v := range headers {
			msg.Headers = append(msg.Headers, kafkago.Header{Key: k, Value: []byte(v)})
		}
	}

	return w.WriteMessages(ctx, msg)
}

func (c *Client) Close() {
	c.writersMu.Lock()
	defer c.writersMu.Unlock()
	for _, w := range c.writers {
		w.Close()
	}
	if c.trans != nil {
		c.trans.CloseIdleConnections()
	}
}

type GroupDetail struct {
	ID, State string
	Members   []kafkago.DescribeGroupsResponseMember
	Offsets   []GroupOffset
	TotalLag  int64
}

type GroupOffset struct {
	Topic        string
	Partition    int
	GroupOffset  int64
	LatestOffset int64
	Lag          int64
}

func (c *Client) ListGroups(ctx context.Context) ([]kafkago.ListGroupsResponseGroup, error) {
	resp, err := c.api.ListGroups(ctx, &kafkago.ListGroupsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", MapError(err))
	}
	return resp.Groups, nil
}

func (c *Client) GetGroupDetail(ctx context.Context, groupID string) (*GroupDetail, error) {
	desc, err := c.api.DescribeGroups(ctx, &kafkago.DescribeGroupsRequest{GroupIDs: []string{groupID}})
	if err != nil {
		return nil, fmt.Errorf("failed to describe group %s: %w", groupID, MapError(err))
	}
	if len(desc.Groups) == 0 {
		return nil, fmt.Errorf("group not found")
	}
	g := desc.Groups[0]

	detail := &GroupDetail{ID: groupID, State: g.GroupState}
	for _, m := range g.Members {
		detail.Members = append(detail.Members, m)
	}

	offFetch, err := c.api.OffsetFetch(ctx, &kafkago.OffsetFetchRequest{GroupID: groupID})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch offsets for group %s: %w", groupID, err)
	}

	var totalLag int64
	
	for topicName, partitions := range offFetch.Topics {
		latestMap := make(map[int]int64)

		for i := 0; i < len(partitions); i += DefaultBatchSize {
			end := i + DefaultBatchSize
			if end > len(partitions) {
				end = len(partitions)
			}
			
			batchPartitions := partitions[i:end]
			reqs := make([]kafkago.OffsetRequest, len(batchPartitions))
			for j, p := range batchPartitions {
				reqs[j] = kafkago.OffsetRequest{Partition: p.Partition, Timestamp: kafkago.LastOffset}
			}
			
			latestResp, err := c.api.ListOffsets(ctx, &kafkago.ListOffsetsRequest{
				Addr:   kafkago.TCP(c.Brokers[0]),
				Topics: map[string][]kafkago.OffsetRequest{topicName: reqs},
			})
			
			if err == nil {
				if po, ok := latestResp.Topics[topicName]; ok {
					for _, o := range po {
						latestMap[o.Partition] = o.LastOffset
					}
				}
			}
		}

		for _, p := range partitions {
			latest := latestMap[p.Partition]
			lag := latest - p.CommittedOffset
			if lag < 0 {
				lag = 0
			}
			totalLag += lag
			detail.Offsets = append(detail.Offsets, GroupOffset{
				Topic: topicName, Partition: p.Partition, GroupOffset: p.CommittedOffset, LatestOffset: latest, Lag: lag,
			})
		}
	}
	detail.TotalLag = totalLag
	return detail, nil
}

func (c *Client) CommitOffset(ctx context.Context, groupID, topic string, partition int, offset int64) error {
	if !c.writeEnabled {
		return ErrReadOnly
	}
	resp, err := c.api.OffsetCommit(ctx, &kafkago.OffsetCommitRequest{
		Addr:         kafkago.TCP(c.Brokers[0]),
		GroupID:      groupID,
		GenerationID: -1,
		Topics: map[string][]kafkago.OffsetCommit{
			topic: {{Partition: partition, Offset: offset}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit offset: %w", err)
	}
	
	if topicResp, ok := resp.Topics[topic]; ok {
		for _, p := range topicResp {
			if p.Partition == partition && p.Error != nil {
				return fmt.Errorf("offset commit error for partition %d: %w", partition, p.Error)
			}
		}
	}
	return nil
}

type ConfigEntry struct {
	Name, Value         string
	IsDefault, ReadOnly bool
}

func (c *Client) GetTopicConfig(ctx context.Context, name string) ([]ConfigEntry, error) {
	resp, err := c.api.DescribeConfigs(ctx, &kafkago.DescribeConfigsRequest{
		Addr: kafkago.TCP(c.Brokers[0]),
		Resources: []kafkago.DescribeConfigRequestResource{{ResourceType: kafkago.ResourceTypeTopic, ResourceName: name}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe configs for topic %s: %w", name, err)
	}
	if len(resp.Resources) == 0 {
		return nil, fmt.Errorf("no config found")
	}
	res := resp.Resources[0]
	if res.Error != nil {
		return nil, fmt.Errorf("config error for topic %s: %w", name, res.Error)
	}
	entries := make([]ConfigEntry, 0, len(res.ConfigEntries))
	for _, e := range res.ConfigEntries {
		entries = append(entries, ConfigEntry{Name: e.ConfigName, Value: e.ConfigValue, IsDefault: e.IsDefault, ReadOnly: e.ReadOnly})
	}
	return entries, nil
}

func (c *Client) UpdateTopicConfig(ctx context.Context, topic, key, value string) error {
	if !c.writeEnabled {
		return ErrReadOnly
	}
	_, err := c.api.IncrementalAlterConfigs(ctx, &kafkago.IncrementalAlterConfigsRequest{
		Addr: kafkago.TCP(c.Brokers[0]),
		Resources: []kafkago.IncrementalAlterConfigsRequestResource{{
			ResourceType: kafkago.ResourceTypeTopic, ResourceName: topic,
			Configs: []kafkago.IncrementalAlterConfigsRequestConfig{{Name: key, Value: value, ConfigOperation: kafkago.ConfigOperationSet}},
		}},
	})
	if err != nil {
		return fmt.Errorf("failed to update config for topic %s: %w", topic, err)
	}
	return nil
}
