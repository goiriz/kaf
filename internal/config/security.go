package config

import "strings"

var DefaultRedactKeys = []string{"password", "secret", "token", "api_key", "authorization", "key"}

func ShouldRedact(key string, keys []string) bool {
	allKeys := make([]string, 0, len(keys)+len(DefaultRedactKeys))
	allKeys = append(allKeys, keys...)
	allKeys = append(allKeys, DefaultRedactKeys...)

	lowerKey := strings.ToLower(key)
	for _, k := range allKeys {
		k = strings.ToLower(k)
		// Exact match is always redacted
		if lowerKey == k {
			return true
		}
		// Redact if it contains common sensitive prefixes/suffixes with separators
		if strings.HasPrefix(lowerKey, k+"_") || strings.HasSuffix(lowerKey, "_"+k) || strings.Contains(lowerKey, "_"+k+"_") ||
			strings.HasPrefix(lowerKey, k+".") || strings.HasSuffix(lowerKey, "."+k) || strings.Contains(lowerKey, "."+k+".") ||
			strings.HasPrefix(lowerKey, k+"-") || strings.HasSuffix(lowerKey, "-"+k) || strings.Contains(lowerKey, "-"+k+"-") {
			return true
		}
	}
	return false
}
