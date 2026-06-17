package main

import (
	"github.com/wengti0608/golang-docker-postgres/router"
)

func main() {
	r := router.New()
	r.Run(":8080")
}
