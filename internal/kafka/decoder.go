package kafka

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/linkedin/goavro/v2"
	"github.com/riferrei/srclient"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
)

// Decoder defines how to transform raw bytes into a displayable string.
type Decoder interface {
	Decode(topic string, value []byte) string
	Errors() []string
	Name() string
}

// DefaultDecoder handles JSON and plain text.
type DefaultDecoder struct{}

func (d DefaultDecoder) Name() string { return "AUTO" }

func (d DefaultDecoder) Decode(topic string, value []byte) string {
	if len(value) == 0 {
		return ""
	}

	// Detect JSON
	if value[0] == '{' || value[0] == '[' {
		var jsonObj interface{}
		if err := json.Unmarshal(value, &jsonObj); err == nil {
			if formatted, err := json.MarshalIndent(jsonObj, "", "  "); err == nil {
				return string(formatted)
			}
		}
	}

	return string(value)
}

func (d DefaultDecoder) Errors() []string { return nil }

// HexDecoder for binary data.
type HexDecoder struct{}

func (d HexDecoder) Name() string { return "HEX" }

func (d HexDecoder) Decode(topic string, value []byte) string {
	return fmt.Sprintf("%x", value)
}

func (d HexDecoder) Errors() []string { return nil }

type SRConfig struct {
	URL      string
	User     string
	Pass     string
	Insecure bool
}

// SRDecoder handles Confluent Wire Format (Magic Byte 0 + SchemaID + Avro/Protobuf Data)
type SRDecoder struct {
	client      *srclient.SchemaRegistryClient
	fallback    Decoder
	cache       sync.Map // map[int]interface{} (can be *goavro.Codec or *desc.MessageDescriptor)
	localProtos map[string]*desc.MessageDescriptor // name -> descriptor
	mappings    map[string]string // topic -> message name
	loadErrors  []string
}

func NewSRDecoder(cfg SRConfig, protoPaths []string, mappings map[string]string) Decoder {
	if cfg.URL == "" && len(protoPaths) == 0 {
		return DefaultDecoder{}
	}

	var srClient *srclient.SchemaRegistryClient
	if cfg.URL != "" {
		srClient = srclient.CreateSchemaRegistryClient(cfg.URL)
		if cfg.User != "" || cfg.Pass != "" {
			srClient.SetCredentials(cfg.User, cfg.Pass)
		}
		srClient.SetTimeout(10 * time.Second)
	}
	
	d := &SRDecoder{
		client:      srClient,
		fallback:    DefaultDecoder{},
		localProtos: make(map[string]*desc.MessageDescriptor),
		mappings:    mappings,
	}
	
	if len(protoPaths) > 0 {
		d.loadLocalProtos(protoPaths)
	}
	
	return d
}

func (d *SRDecoder) Errors() []string {
	return d.loadErrors
}

func (d *SRDecoder) Name() string { return "PROTO/AVRO" }

func (d *SRDecoder) loadLocalProtos(paths []string) {
	var protoFiles []string
	var absPaths []string
	for _, p := range paths {
		absP, err := filepath.Abs(p)
		if err == nil {
			absPaths = append(absPaths, absP)
			filepath.Walk(absP, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && filepath.Ext(path) == ".proto" {
					rel, err := filepath.Rel(absP, path)
					if err == nil {
						protoFiles = append(protoFiles, rel)
					}
				}
				return nil
			})
		}
	}

	if len(protoFiles) == 0 {
		return
	}

	parser := protoparse.Parser{
		ImportPaths: absPaths,
	}
	fds, err := parser.ParseFiles(protoFiles...)
	if err == nil {
		for _, fd := range fds {
			for _, md := range fd.GetMessageTypes() {
				d.localProtos[md.GetName()] = md
				d.localProtos[md.GetFullyQualifiedName()] = md
			}
		}
	} else {
		d.loadErrors = append(d.loadErrors, fmt.Sprintf("Proto Parse Error: %v", err))
	}
}

func (d *SRDecoder) Decode(topic string, value []byte) string {
	// Confluent Wire Format: Magic Byte (0) + 4 bytes Schema ID + Data
	if len(value) > 5 && value[0] == 0 {
		schemaID := int(binary.BigEndian.Uint32(value[1:5]))

		// 1. Check Cache
		cached, ok := d.cache.Load(schemaID)
		if ok {
			switch c := cached.(type) {
			case *goavro.Codec:
				return d.decodeAvro(schemaID, c, value[5:])
			case *desc.MessageDescriptor:
				return d.decodeProto(schemaID, c, value[5:])
			}
		}

		if d.client != nil {
			// 2. Fetch Schema & Create Codec/Descriptor
			schema, err := d.client.GetSchema(schemaID)
			if err == nil && schema != nil {
				sType := srclient.Avro
				if schema.SchemaType() != nil {
					sType = *schema.SchemaType()
				}

				switch sType {
				case srclient.Avro:
					c, err := goavro.NewCodec(schema.Schema())
					if err == nil {
						d.cache.Store(schemaID, c)
						return d.decodeAvro(schemaID, c, value[5:])
					}
				case srclient.Protobuf:
					descriptor, err := d.parseProtoSchema(schema.Schema())
					if err == nil {
						d.cache.Store(schemaID, descriptor)
						return d.decodeProto(schemaID, descriptor, value[5:])
					}
				case srclient.Json:
					var jsonObj interface{}
					if err := json.Unmarshal(value[5:], &jsonObj); err == nil {
						if formatted, err := json.MarshalIndent(jsonObj, "", "  "); err == nil {
							return fmt.Sprintf("[JSONSCHEMA Schema:%d]\n%s", schemaID, string(formatted))
						}
					}
					return fmt.Sprintf("[JSONSCHEMA Schema:%d]\n%s", schemaID, string(value[5:]))
				}
			}
		}

		// Fallback for SR match but decode failure
		if schemaID > 0 {
			return fmt.Sprintf("[SchemaID:%d] (Binary Payload)\n%x", schemaID, value[5:])
		}
	}

	// 4. Try Strict Mapping Match
	if msgName, ok := d.mappings[topic]; ok {
		if md, ok := d.localProtos[msgName]; ok {
			msg := dynamic.NewMessage(md)
			if err := msg.Unmarshal(value); err == nil {
				if b, err := msg.MarshalJSONIndent(); err == nil {
					return fmt.Sprintf("[PROTO:%s]\n%s", md.GetName(), string(b))
				}
			}
			return fmt.Sprintf("[PROTO:%s] (Decode Error)\n%x", msgName, value)
		}
		return fmt.Sprintf("[PROTO:%s] (Schema Not Found)\n%x", msgName, value)
	}

	// 5. Try Local Protobuf Match (for raw messages without Wire Format)
	if len(d.localProtos) > 0 && len(value) > 0 {
		for _, md := range d.localProtos {
			msg := dynamic.NewMessage(md)
			// Try decoding raw payload
			if err := msg.Unmarshal(value); err == nil {
				// To avoid false positives (since Protobuf is very permissive),
				// we check if at least one field was set.
				if len(msg.GetKnownFields()) > 0 {
					if b, err := msg.MarshalJSONIndent(); err == nil {
						return fmt.Sprintf("[PROTO:%s]\n%s", md.GetName(), string(b))
					}
				}
			}
		}
	}

	return d.fallback.Decode(topic, value)
}

func (d *SRDecoder) decodeAvro(schemaID int, codec *goavro.Codec, data []byte) string {
	native, _, err := codec.NativeFromBinary(data)
	if err == nil {
		if formatted, err := json.MarshalIndent(native, "", "  "); err == nil {
			return fmt.Sprintf("[AVRO Schema:%d]\n%s", schemaID, string(formatted))
		}
	}
	return fmt.Sprintf("[AVRO Schema:%d] (Decode error: %v)\n%x", schemaID, err, data)
}

func (d *SRDecoder) decodeProto(schemaID int, md *desc.MessageDescriptor, data []byte) string {
	// Confluent Protobuf Wire Format includes a message index array after the ID
	// For simplicity, we skip the index array (usually [0]) and try to decode.
	
	payload := data
	if len(data) > 0 {
		// Basic skip of index array (usually just [0] which is 0x00 in zigzag/varint)
		if data[0] == 0 {
			payload = data[1:]
		}
	}

	msg := dynamic.NewMessage(md)
	if err := msg.Unmarshal(payload); err == nil {
		if b, err := msg.MarshalJSONIndent(); err == nil {
			return fmt.Sprintf("[PROTOBUF Schema:%d]\n%s", schemaID, string(b))
		}
	}
	
	// If it fails, try with the original data (maybe no index array)
	if err := msg.Unmarshal(data); err == nil {
		if b, err := msg.MarshalJSONIndent(); err == nil {
			return fmt.Sprintf("[PROTOBUF Schema:%d]\n%s", schemaID, string(b))
		}
	}

	return fmt.Sprintf("[PROTOBUF Schema:%d] (Binary Payload)\n%x", schemaID, data)
}

func (d *SRDecoder) parseProtoSchema(schema string) (*desc.MessageDescriptor, error) {
	parser := protoparse.Parser{
		Accessor: protoparse.FileContentsFromMap(map[string]string{
			"schema.proto": schema,
		}),
	}
	fds, err := parser.ParseFiles("schema.proto")
	if err != nil {
		return nil, err
	}
	
	// Return the last message descriptor in the file (often the main one)
	msgs := fds[0].GetMessageTypes()
	if len(msgs) == 0 {
		return nil, fmt.Errorf("no message types in proto")
	}
	return msgs[len(msgs)-1], nil
}
