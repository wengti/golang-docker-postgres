package router

import (
	"github.com/gin-gonic/gin"
	"github.com/wengti0608/golang-docker-postgres/handlers"
)

// New builds the Gin engine and registers all routes against the given handler.
func New(h *handlers.Handler) *gin.Engine {
	r := gin.Default()

	r.GET("/", h.Hello)
	r.POST("/users", h.CreateUser)

	return r
}
