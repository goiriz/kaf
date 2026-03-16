package kafka

import (
	"encoding/binary"
	"testing"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

func TestDefaultDecoder(t *testing.T) {
	d := DefaultDecoder{}
	
	t.Run("Plain Text", func(t *testing.T) {
		input := []byte("hello world")
		output := d.Decode("test", input)
		if output != "hello world" {
			t.Errorf("Expected 'hello world', got %s", output)
		}
	})

	t.Run("Pretty JSON", func(t *testing.T) {
		input := []byte(`{"a":1,"b":2}`)
		output := d.Decode("test", input)
		if !contains(output, `"a": 1`) || !contains(output, "\n") {
			t.Errorf("Expected pretty JSON, got %s", output)
		}
	})
}

func TestHexDecoder(t *testing.T) {
	d := HexDecoder{}
	input := []byte{0xde, 0xad, 0xbe, 0xef}
	output := d.Decode("test", input)
	if output != "deadbeef" {
		t.Errorf("Expected 'deadbeef', got %s", output)
	}
}

func TestSRDecoder_Fallback(t *testing.T) {
	d := NewSRDecoder(SRConfig{URL: ""}, []string{}, nil) // Empty URL leads to DefaultDecoder fallback
	if _, ok := d.(DefaultDecoder); !ok {
		t.Errorf("Expected DefaultDecoder fallback for empty URL, got %T", d)
	}
}

func TestSRDecoder_WireFormatDetection(t *testing.T) {
	d := &SRDecoder{
		fallback: DefaultDecoder{},
	}
	
	t.Run("Invalid Wire Format", func(t *testing.T) {
		input := []byte{0x01, 0x00, 0x00, 0x00, 0x01, 'h', 'e', 'l', 'l', 'o'}
		output := d.Decode("test", input)
		// Should fallback since first byte is not 0
		if output != string(input) {
			t.Errorf("Expected fallback decode, got %s", output)
		}
	})

	t.Run("Magic Byte But No Client", func(t *testing.T) {
		// Magic byte 0 + Schema ID 1 + data
		input := []byte{0x00, 0x00, 0x00, 0x00, 0x01, 'd', 'a', 't', 'a'}
		output := d.Decode("test", input)
		if !contains(output, "[SchemaID:1]") {
			t.Errorf("Expected SchemaID mention, got %s", output)
		}
	})
}

func TestSRDecoder_ProtoDecoding(t *testing.T) {
	d := &SRDecoder{
		localProtos: make(map[string]*desc.MessageDescriptor),
	}

	// Simple proto schema
	schema := `syntax = "proto3"; message User { string name = 1; int32 age = 2; }`
	md, err := d.parseProtoSchema(schema)
	if err != nil {
		t.Fatalf("Failed to parse schema: %v", err)
	}

	// Store in cache as schemaID 10
	d.cache.Store(10, md)

	// Create a dynamic message and marshal it
	msg := dynamic.NewMessage(md)
	msg.SetFieldByName("name", "John")
	msg.SetFieldByName("age", int32(30))
	protoBytes, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal proto: %v", err)
	}

	// Confluent Wire Format: Magic (0) + SchemaID (10) + Index Array (0) + Data
	input := make([]byte, 5)
	input[0] = 0
	binary.BigEndian.PutUint32(input[1:], 10)
	input = append(input, 0) // Index array [0]
	input = append(input, protoBytes...)

	output := d.Decode("test", input)
	if !contains(output, "[PROTOBUF Schema:10]") || !contains(output, `"John"`) || !contains(output, "30") {
		t.Errorf("Expected protobuf decoded output, got %s", output)
	}
}

// Internal helper for tests
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
