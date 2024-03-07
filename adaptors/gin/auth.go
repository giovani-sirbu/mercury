package ginAdaptors

import (
	"github.com/gin-gonic/gin"
	"mercury/auth"
	"net/http"
	"strings"
)

func IsAuth(c *gin.Context) {
	token := strings.Split(c.Request.Header["Authorization"][0], " ")[1]
	err := auth.VerifyToken(token)

	if err != nil {
		// User is not logged in
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "UNAUTHORIZED"})
		return
	}

	// Continue down the chain, user is logged in
	c.Next()
}
