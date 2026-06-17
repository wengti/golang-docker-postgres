package main

import (
	"context"
	"log"

	"github.com/joho/godotenv"
	"github.com/wengti0608/golang-docker-postgres/db"
	"github.com/wengti0608/golang-docker-postgres/router"
)

func main() {
	// Load .env into the process environment. Not fatal if missing, so real
	// environment variables (e.g. in production) still work.
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on existing environment variables")
	}

	// context.Background() is the root context (no timeout/cancellation),
	// used here because startup has no parent request context to inherit.
	pool, err := db.Connect(context.Background())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("connected to database")

	// Ensure the schema exists before serving traffic. Idempotent, so it is
	// safe to run on every startup.
	if err := db.Migrate(context.Background(), pool); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("database schema ready")

	r := router.New()
	r.Run(":8080")
}
