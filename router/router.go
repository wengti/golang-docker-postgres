package router

import (
	"github.com/gin-gonic/gin"
	"github.com/wengti0608/golang-docker-postgres/handlers"
)

// New builds the Gin engine and registers all routes.
func New() *gin.Engine {
	r := gin.Default()

	r.GET("/", handlers.Hello)

	return r
}
