package ginAdaptors

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/giovani-sirbu/mercury/auth"
)

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
	token, ok := extractBearerToken(c)
	if !ok {
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	if err := auth.VerifyToken(token); err != nil {
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
			c.Abort()
			Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
			return
		}
		if userInfo.Id != userId && userInfo.Role != "admin" {
			c.Abort()
			Response(c, http.StatusForbidden, "ACCESS_FORBIDDEN")
			return
		}
	}

	c.Next()
}

func IsAdmin(c *gin.Context) {
	token, ok := extractBearerToken(c)
	if !ok {
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	if err := auth.VerifyToken(token); err != nil {
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	userInfo, err := auth.ParseToken(token)
	if err != nil {
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	if userInfo.Role != "admin" {
		c.Abort()
		Response(c, http.StatusForbidden, "ACCESS_FORBIDDEN")
		return
	}

	c.Next()
}
