package ginAdaptors

import (
	"github.com/gin-gonic/gin"
	"github.com/giovani-sirbu/mercury/auth"
	"net/http"
	"strconv"
	"strings"
)

func stringToUint(s string) uint {
	i, _ := strconv.Atoi(s)
	return uint(i)
}

func IsAuth(c *gin.Context) {
	authHeader := c.Request.Header["Authorization"]
	userId := stringToUint(c.Param("userId"))

	if len(authHeader) < 1 {
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	token := strings.Split(c.Request.Header["Authorization"][0], " ")[1]
	err := auth.VerifyToken(token)

	if err != nil {
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	// if userId exist in url, compare it with userId stored in token and return error if different
	if userId != 0 {
		userInfo, _ := auth.ParseToken(token)
		if userInfo.Id != userId {
			c.Abort()
			Response(c, http.StatusForbidden, "ACCESS_FORBIDDEN")
			return
		}
	}

	// Continue down the chain, user is logged in
	c.Next()
}

func IsAdmin(c *gin.Context) {
	authHeader := c.Request.Header["Authorization"]

	if len(authHeader) < 1 {
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	token := strings.Split(c.Request.Header["Authorization"][0], " ")[1]
	err := auth.VerifyToken(token)

	if err != nil {
		c.Abort()
		Response(c, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	userInfo, _ := auth.ParseToken(token)
	if userInfo.Role != "admin" {
		c.Abort()
		Response(c, http.StatusForbidden, "ACCESS_FORBIDDEN")
		return
	}

	// Continue down the chain, user is logged in
	c.Next()
}
