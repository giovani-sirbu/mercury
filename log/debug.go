package log

import (
	"fmt"
	"os"
	"sync"
)

// debugEnabled is resolved once on first call, not at package init. Reading
// DEBUG via os.Getenv on every Debug call shows up in profiles on the hot path
// (managePrices / manageTrades / events.Run all log per tick), so we still
// memoize — but we defer the read until first use so godotenv.Load() (which
// runs inside main(), after package-level initializers) has a chance to
// populate the env from .env first. Previously this was a `var =` at package
// scope, which evaluated before main and saw an empty DEBUG, leaving the flag
// permanently false in containers whose compose spec doesn't pre-inject DEBUG.
//
// Trade-off: changing DEBUG at runtime still requires a restart. Acceptable —
// services already restart for any other config change.
var (
	debugOnce    sync.Once
	debugEnabled bool
)

func Debug(msg ...any) {
	debugOnce.Do(func() {
		debugEnabled = os.Getenv("DEBUG") == "true"
	})
	if debugEnabled {
		fmt.Println(msg...)
	}
}
