// Package config provides startup-time guards that every service should run
// against its env-derived config before serving traffic. Failing fast here
// produces a single panic log line in CI / on container start, which is
// much easier to debug than a service that comes up healthy and then
// errors out on the first cache call or DB query.
package config

import (
	"fmt"
	"os"
	"strings"
)

// RequireEnv panics if any of keys is empty in the current environment.
// Call this once at service startup with the list of variables the service
// cannot run without. Optional values stay where they are (read via
// os.Getenv with a sensible default).
//
// Example:
//
//	config.RequireEnv("MESSAGEBUS_DSN", "REDIS_URL", "API_SECRET")
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

// GuardCORSWildcard panics if origins contains a wildcard ("*") and GO_ENV
// is "production". The combination is almost never intentional in prod —
// the most common cause is a copy-paste of a dev .env, and the cost of a
// late-bound CORS mistake is high.
//
// In non-production environments wildcards are allowed; this is a startup
// guard, not a hard ban.
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
