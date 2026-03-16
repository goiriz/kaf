package common

import (
	"strings"
	"testing"
)

func TestFormatMessageValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "Plain text remains same",
			input:    "hello world",
			contains: "hello world",
		},
		{
			name:     "Valid JSON is pretty-printed",
			input:    `{"a":1,"b":"test"}`,
			contains: "  \"a\": 1",
		},
		{
			name:     "Invalid JSON remains same",
			input:    `{"a":1,`,
			contains: `{"a":1,`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := FormatMessageValue([]byte(tt.input))
			if !strings.Contains(res, tt.contains) {
				t.Errorf("Expected result to contain %q, got %q", tt.contains, res)
			}
		})
	}
}

func TestMatchesFilter(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		val    string
		filter string
		want   bool
	}{
		{"Empty filter matches all", "key", "val", "", true},
		{"Matches value", "key", "hello world", "world", true},
		{"Matches key", "user-123", "data", "123", true},
		{"Case insensitive match", "Key", "VALUE", "value", true},
		{"No match", "abc", "def", "xyz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesFilter([]byte(tt.key), []byte(tt.val), tt.filter); got != tt.want {
				t.Errorf("MatchesFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}
