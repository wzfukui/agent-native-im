package utils

import (
	"context"
	"time"
)

// DefaultTimeout is the default timeout for database operations
const DefaultTimeout = 30 * time.Second

// ShortTimeout is used for quick operations
const ShortTimeout = 5 * time.Second

// LongTimeout is used for batch operations
const LongTimeout = 60 * time.Second

// ContextWithTimeout creates a context with the default timeout
func ContextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), DefaultTimeout)
}

// ContextWithShortTimeout creates a context with a short timeout
func ContextWithShortTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), ShortTimeout)
}

// ContextWithLongTimeout creates a context with a long timeout
func ContextWithLongTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), LongTimeout)
}

// ContextWithCustomTimeout creates a context with a custom timeout
func ContextWithCustomTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}