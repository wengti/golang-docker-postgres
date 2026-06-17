package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Hello responds with a simple hello world message.
func (h *Handler) Hello(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "hello world!",
	})
}
