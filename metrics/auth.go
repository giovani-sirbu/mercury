package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// AuthLoginAttempts counts login attempts, partitioned by result and reason.
var AuthLoginAttempts = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "auth_login_attempts_total",
		Help: "Login attempts received by hellenes, partitioned by outcome.",
	},
	[]string{"service", "result", "reason"}, // result: success | fail
)

// JWTValidationFailures counts failed JWT validations in the gin middleware:
// missing (no header), invalid (VerifyToken failed), forbidden (another user's resource).
var JWTValidationFailures = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "auth_jwt_validation_failures_total",
		Help: "Failed JWT validations in the IsAuth/IsAdmin middleware.",
	},
	[]string{"service", "reason"}, // reason: missing | invalid | forbidden
)
