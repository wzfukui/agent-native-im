package utils

import (
	"fmt"
	"log"
	"runtime/debug"
)

// SafeGo runs a goroutine with panic recovery
func SafeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				log.Printf("PANIC in goroutine [%s]: %v\nStack trace:\n%s", name, r, stack)
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
						log.Printf("PANIC in goroutine [%s] (retry %d/%d): %v\nStack trace:\n%s",
							name, retries+1, maxRetries, r, stack)
						retries++
					}
				}()
				fn()
				// If function exits normally, reset retries
				retries = maxRetries
			}()
		}
		if retries >= maxRetries {
			log.Printf("Goroutine [%s] exceeded max retries (%d), not restarting", name, maxRetries)
		}
	}()
}

// RecoverPanic is a defer function for recovering from panics
func RecoverPanic(context string) {
	if r := recover(); r != nil {
		stack := debug.Stack()
		err := fmt.Errorf("panic in %s: %v", context, r)
		log.Printf("PANIC RECOVERED [%s]: %v\nStack trace:\n%s", context, err, stack)
	}
}