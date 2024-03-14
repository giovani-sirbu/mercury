package ginAdaptors

import "github.com/gin-gonic/gin"

func Init(c *gin.Context) {
	gin.Default()

	c.Next()
}
