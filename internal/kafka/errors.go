package kafka

import (
	"errors"
	"net"
	"strings"

	kafkago "github.com/segmentio/kafka-go"
)

var (
	ErrTopicNotFound      = errors.New("topic not found")
	ErrGroupNotFound      = errors.New("consumer group not found")
	ErrAuthenticationFail = errors.New("authentication failed (check SASL credentials)")
	ErrConnectionRefused  = errors.New("connection refused (check broker address)")
	ErrTimeout            = errors.New("operation timed out")
	ErrReadOnly           = errors.New("client is in READ-ONLY mode (use --write flag to enable)")
)

// MapError translates raw kafka-go errors to our domain errors if possible.
func MapError(err error) error {
	if err == nil {
		return nil
	}

	// 1. Direct matches using errors.Is
	if errors.Is(err, kafkago.UnknownTopicOrPartition) {
		return ErrTopicNotFound
	}

	// 2. Network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return ErrTimeout
		}
		// Connection refused often presents as net.OpError
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			if strings.Contains(opErr.Error(), "connection refused") {
				return ErrConnectionRefused
			}
		}
	}

	// 3. Fallback for SASL/Auth (kafka-go doesn't always expose these as typed errors)
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "authentication") || strings.Contains(errStr, "sasl") || strings.Contains(errStr, "denied") {
		return ErrAuthenticationFail
	}

	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return ErrTimeout
	}

	return err
}
