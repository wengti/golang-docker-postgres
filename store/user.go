package store

import (
	"context"
	"time"
)

// User mirrors a row in the users table. The json tags control how it is
// serialized in API responses.
type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateUser inserts a new user and returns the stored row, including the
// database-generated id and created_at.
func (s *Store) CreateUser(ctx context.Context, name, email string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (name, email)
		 VALUES ($1, $2)
		 RETURNING id, name, email, created_at`,
		name, email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}
