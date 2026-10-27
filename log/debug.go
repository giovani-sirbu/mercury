package log

import (
	"fmt"
	"os"
	"sync"
)

// debugEnabled is resolved once on the first Debug call, after godotenv has
// loaded the env, and memoised off the per-tick hot path; changing DEBUG
// still needs a restart.
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
