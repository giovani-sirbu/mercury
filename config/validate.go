// Package config provides startup guards a service runs on its env config before serving.
package config

import (
	"fmt"
	"os"
	"strings"
)

// RequireEnv panics when any of keys is empty in the environment.
func RequireEnv(keys ...string) {
	var missing []string
	for _, k := range keys {
		if strings.TrimSpace(os.Getenv(k)) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		panic(fmt.Sprintf("config: required env vars missing: %s", strings.Join(missing, ", ")))
	}
}

// GuardCORSWildcard panics on a wildcard origin when GO_ENV is production.
func GuardCORSWildcard(origins []string) {
	if !strings.EqualFold(os.Getenv("GO_ENV"), "production") {
		return
	}
	for _, o := range origins {
		if strings.TrimSpace(o) == "*" {
			panic("config: CORS wildcard origin is not permitted in production")
		}
	}
}
