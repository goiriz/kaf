package common

import (
	"strings"
	"testing"

	"github.com/goiriz/kaf/internal/config"
)

func TestRedactJSON(t *testing.T) {
	keys := []string{"password", "secret_key"}
	
	t.Run("JSON Redaction", func(t *testing.T) {
		input := `{"user": "admin", "password": "123", "nested": {"secret_key": "abc", "other": "val"}}`
		output := RedactJSON(input, keys)
		
		if !strings.Contains(output, `"password": "[REDACTED]"`) {
			t.Errorf("Password should be redacted in JSON. Got: %s", output)
		}
		if !strings.Contains(output, `"secret_key": "[REDACTED]"`) {
			t.Errorf("Nested secret_key should be redacted. Got: %s", output)
		}
		if !strings.Contains(output, `"user": "admin"`) {
			t.Errorf("Non-sensitive data should remain. Got: %s", output)
		}
	})

	t.Run("Plain Text Fallback", func(t *testing.T) {
		input := "Login with password=123 and secret_key: abc"
		output := RedactJSON(input, keys)
		
		if !strings.Contains(output, "password=[REDACTED]") {
			t.Errorf("Password should be redacted in plain text. Got: %s", output)
		}
		if !strings.Contains(output, "secret_key:[REDACTED]") {
			t.Errorf("secret_key should be redacted in plain text. Got: %s", output)
		}
	})

	t.Run("Default Redact Keys", func(t *testing.T) {
		input := `{"token": "xyz"}`
		output := RedactJSON(input, nil) // No keys provided, use defaults
		
		if !strings.Contains(output, `"token": "[REDACTED]"`) {
			t.Errorf("Token (default key) should be redacted. Got: %s", output)
		}
	})
}

func TestShouldRedact(t *testing.T) {
	if !config.ShouldRedact("password", nil) {
		t.Error("ShouldRedact should match 'password' by default")
	}
	if !config.ShouldRedact("Authorization", nil) {
		t.Error("ShouldRedact should match 'authorization' default key")
	}
	if config.ShouldRedact("username", nil) {
		t.Error("ShouldRedact should NOT match 'username' by default")
	}
}
