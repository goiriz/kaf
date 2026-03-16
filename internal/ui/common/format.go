package common

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/goiriz/kaf/internal/config"
)

// FormatMessageValue detects JSON and applies indentation if valid.
func FormatMessageValue(value []byte) string {
	var jsonObj interface{}
	if err := json.Unmarshal(value, &jsonObj); err == nil {
		if formatted, err := json.MarshalIndent(jsonObj, "", "  "); err == nil {
			return string(formatted)
		}
	}
	return string(value)
}

// RedactJSON parses a JSON string and masks values for the given keys.
func RedactJSON(input string, keys []string) string {
	// Merge provided keys with defaults
	allKeys := make([]string, 0, len(keys)+len(config.DefaultRedactKeys))
	allKeys = append(allKeys, keys...)
	allKeys = append(allKeys, config.DefaultRedactKeys...)

	if len(allKeys) == 0 {
		return input
	}

	var data interface{}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		// FALLBACK: Not JSON, apply plain text redaction
		return redactPlainText(input, allKeys)
	}

	redacted := redactRecursive(data, allKeys)

	if b, err := json.MarshalIndent(redacted, "", "  "); err == nil {
		return string(b)
	}
	return redactPlainText(input, allKeys)
}

func redactPlainText(input string, keys []string) string {
	output := input
	for _, k := range keys {
		// Simple but safe: mask "key=value" or "key: value" or "key:value" patterns
		// This is a basic protection for logs/plain text.
		re := strings.NewReplacer(
			k+"=", k+"=[REDACTED]",
			k+":", k+":[REDACTED]",
			k+" =", k+"= [REDACTED]",
			k+" :", k+": [REDACTED]",
		)
		output = re.Replace(output)
	}
	return output
}

func redactRecursive(data interface{}, keys []string) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if config.ShouldRedact(key, keys) {
				v[key] = "[REDACTED]"
			} else {
				v[key] = redactRecursive(val, keys)
			}
		}
	case []interface{}:
		for i, val := range v {
			v[i] = redactRecursive(val, keys)
		}
	}
	return data
}
// Remove old private shouldRedact

// MatchesFilter checks if a message (key/value) matches the search term without unnecessary allocations.
func MatchesFilter(key, value []byte, filter string) bool {
	if filter == "" {
		return true
	}
	
	fBytes := []byte(strings.ToLower(filter))
	
	// Fast path: avoid string allocation and ToLower on the whole payload
	// bytes.Contains doesn't do case-insensitive, so we do a quick lowercasing of a copy if needed, 
	// but we only lowercase the bytes, not creating huge strings.
	lowerKey := bytes.ToLower(key)
	if bytes.Contains(lowerKey, fBytes) {
		return true
	}
	
	lowerVal := bytes.ToLower(value)
	return bytes.Contains(lowerVal, fBytes)
}

// FilterMatches provides a centralized, case-insensitive string matching utility for lists.
func FilterMatches(input, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(input), strings.ToLower(filter))
}
