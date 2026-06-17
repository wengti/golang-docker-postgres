package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrUserNotFound is returned when an operation targets a user id that does not
// exist. Callers can detect it with errors.Is to map it to a 404 response.
var ErrUserNotFound = errors.New("user not found")

// ErrEmailExists is returned by CreateUser when the email already exists,
// violating the unique constraint. Callers can map it to a 409 response.
var ErrEmailExists = errors.New("email already exists")

// pgUniqueViolation is the PostgreSQL SQLSTATE code for a unique-constraint
// violation. See https://www.postgresql.org/docs/current/errcodes-appendix.html
const pgUniqueViolation = "23505"

// User mirrors a row in the users table. The json tags control how it is
// serialized in API responses.
type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// ListUsers returns all users, ordered by id.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, email, created_at
		 FROM users
		 ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Start with a non-nil empty slice so an empty table serializes as [] in
	// JSON rather than null.
	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	// Iterating with rows.Next() hides errors (e.g. a broken connection
	// mid-stream); rows.Err() surfaces them after the loop.
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
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
		// Translate a unique-constraint violation on email into a domain error
		// the caller can recognize, instead of leaking the raw pgx error.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return User{}, ErrEmailExists
		}
		return User{}, err
	}
	return u, nil
}

// DeleteUser removes the user with the given id. It returns ErrUserNotFound if
// no row matched, so the caller can tell "deleted" apart from "did not exist".
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}

	// Exec succeeds even when it matches zero rows, so inspect how many rows
	// were actually affected to detect a missing id.
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}
