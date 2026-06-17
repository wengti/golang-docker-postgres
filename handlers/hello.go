package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Hello responds with a simple hello world message.
func Hello(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "hello world!",
	})
}
