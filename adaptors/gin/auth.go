package ginAdaptors

import (
	"github.com/gin-gonic/gin"
	"github.com/giovani-sirbu/mercury/auth"
	"net/http"
	"strings"
)

func IsAuth(c *gin.Context) {
	authHeader := c.Request.Header["Authorization"]
	userId := c.Param("userId")

	if len(authHeader) < 1 {
		c.Abort()
		Response(c, http.StatusUnauthorized, Data{Message: "UNAUTHORIZED"})
		return
	}

	token := strings.Split(c.Request.Header["Authorization"][0], " ")[1]
	err := auth.VerifyToken(token)

	if err != nil {
		c.Abort()
		Response(c, http.StatusUnauthorized, Data{Message: "UNAUTHORIZED"})
		return
	}

	// if userId exist in url, compare it with userId stored in token and return error if different
	// TODO - bypass if role is admin/superAdmin
	if len(userId) != 0 {
		userInfo, _ := auth.ParseToken(token)
		if userInfo.Id != userId {
			c.Abort()
			Response(c, http.StatusUnauthorized, Data{Message: "UNAUTHORIZED"})
			return
		}
	}

	// Continue down the chain, user is logged in
	c.Next()
}
