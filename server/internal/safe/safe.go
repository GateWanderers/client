// Package safe provides panic-recovery helpers for goroutines.
package safe

import (
	"log/slog"
	"runtime/debug"
)

// Go runs fn in a new goroutine. If fn panics, the panic is recovered and
// logged with a full stack trace. The goroutine does NOT restart automatically.
func Go(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("goroutine panic recovered",
					"panic", r,
					"stack", string(debug.Stack()),
				)
			}
		}()
		fn()
	}()
}
