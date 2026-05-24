package ginAdaptors

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/giovani-sirbu/mercury/auth"
	"github.com/giovani-sirbu/mercury/metrics"
)

// authServiceName is the value used for the `service` label on the auth
// metrics. Read from the same SERVICE_NAME env var the rest of mercury
// uses for self-identification, falling back to "unknown" so a missing
// env doesn't dump everything into an empty label.
func authServiceName() string {
	if v := os.Getenv("SERVICE_NAME"); v != "" {
		return v
	}
	return "unknown"
}

func stringToUint(s string) uint {
	i, _ := strconv.Atoi(s)
	return uint(i)
}

// extractBearerToken returns the token from an `Authorization: Bearer <token>` header.
// It returns ok=false when the header is missing or malformed so callers can reject
// the request without indexing into an out-of-range slice (the prior version panicked
// on headers that did not contain a space).
func extractBearerToken(c *gin.Context) (string, bool) {
	authHeader := c.Request.Header["Authorization"]
	if len(authHeader) < 1 || authHeader[0] == "" {
		return "", false
	}
	parts := strings.SplitN(authHeader[0], " ", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func IsAuth(c *gin.Context) {
	svc := authServiceName()
	token, ok := extractBearerToken(c)
	if !ok {
		metrics.JWTValidationFailures.WithLabelValues(svc, "missing").Inc()
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	if err := auth.VerifyToken(token); err != nil {
		metrics.JWTValidationFailures.WithLabelValues(svc, "invalid").Inc()
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	// When the route carries a :userId path param, compare it against the token
	// claim so users cannot operate on other users' resources by forging URLs.
	userId := stringToUint(c.Param("userId"))
	if userId != 0 {
		userInfo, err := auth.ParseToken(token)
		if err != nil {
			metrics.JWTValidationFailures.WithLabelValues(svc, "invalid").Inc()
			c.Abort()
			Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
			return
		}
		if userInfo.Id != userId && userInfo.Role != "admin" {
			// Token is valid but targets another user's resource — the
			// strongest cross-tenant-attack signal we have. Tracking it
			// separately from "invalid" so alerts can fire only on this.
			metrics.JWTValidationFailures.WithLabelValues(svc, "forbidden").Inc()
			c.Abort()
			Response(c, http.StatusForbidden, "ACCESS_FORBIDDEN")
			return
		}
	}

	c.Next()
}

func IsAdmin(c *gin.Context) {
	svc := authServiceName()
	token, ok := extractBearerToken(c)
	if !ok {
		metrics.JWTValidationFailures.WithLabelValues(svc, "missing").Inc()
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	if err := auth.VerifyToken(token); err != nil {
		metrics.JWTValidationFailures.WithLabelValues(svc, "invalid").Inc()
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	userInfo, err := auth.ParseToken(token)
	if err != nil {
		metrics.JWTValidationFailures.WithLabelValues(svc, "invalid").Inc()
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	if userInfo.Role != "admin" {
		metrics.JWTValidationFailures.WithLabelValues(svc, "forbidden").Inc()
		c.Abort()
		Response(c, http.StatusForbidden, "ACCESS_FORBIDDEN")
		return
	}

	c.Next()
}
