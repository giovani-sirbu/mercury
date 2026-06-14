package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// AuthLoginAttempts counts every login attempt seen by hellenes,
// partitioned by result + reason. A jump in fail rate with reason
// "invalid_credentials" can mean a credential-stuffing attempt. Same
// rate with "user_not_found" might just be users typoing emails.
//
//	sum by (result, reason) (rate(auth_login_attempts_total[5m]))
//
// gives the breakdown. Burst on a single source IP can be cross-
// referenced via Loki logs in the same time window.
var AuthLoginAttempts = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "auth_login_attempts_total",
		Help: "Login attempts received by hellenes, partitioned by outcome.",
	},
	[]string{"service", "result", "reason"}, // result: success | fail
)

// JWTValidationFailures counts failed JWT validations in the gin
// middleware. Reasons enumerated so suspicious patterns surface
// without scanning logs:
//
//   missing       — Authorization header absent / malformed
//   invalid       — token failed VerifyToken (expired / tampered)
//   forbidden     — token valid but the URL param :userId doesn't
//                   match the token's user (cross-tenant attempt)
//
// A spike in `forbidden` is the strongest signal — it's a successful
// auth where the user is targeting someone else's resources.
var JWTValidationFailures = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "auth_jwt_validation_failures_total",
		Help: "Failed JWT validations in the IsAuth/IsAdmin middleware.",
	},
	[]string{"service", "reason"}, // reason: missing | invalid | forbidden
)
