package utils

import (
	"fmt"
	"log/slog"
	"runtime/debug"
)

// SafeGo runs a goroutine with panic recovery
func SafeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				slog.Error("PANIC in goroutine", "name", name, "error", r, "stack", string(stack))
			}
		}()
		fn()
	}()
}

// SafeGoWithRestart runs a goroutine with panic recovery and automatic restart
func SafeGoWithRestart(name string, fn func(), maxRetries int) {
	go func() {
		retries := 0
		for retries < maxRetries {
			func() {
				defer func() {
					if r := recover(); r != nil {
						stack := debug.Stack()
						slog.Error("PANIC in goroutine",
							"name", name, "retry", retries+1, "max_retries", maxRetries, "error", r, "stack", string(stack))
						retries++
					}
				}()
				fn()
				// If function exits normally, reset retries
				retries = maxRetries
			}()
		}
		if retries >= maxRetries {
			slog.Error("goroutine exceeded max retries, not restarting", "name", name, "max_retries", maxRetries)
		}
	}()
}

// RecoverPanic is a defer function for recovering from panics
func RecoverPanic(context string) {
	if r := recover(); r != nil {
		stack := debug.Stack()
		err := fmt.Errorf("panic in %s: %v", context, r)
		slog.Error("PANIC RECOVERED", "context", context, "error", err, "stack", string(stack))
	}
}