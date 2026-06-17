package handlers

import "github.com/wengti0608/golang-docker-postgres/store"

// Handler holds the dependencies shared by all HTTP handlers (currently the
// store). Handlers are methods on this struct, so they get access to the store
// without relying on package-level globals.
type Handler struct {
	store *store.Store
}

// New returns a Handler wired with the given store.
func New(s *store.Store) *Handler {
	return &Handler{store: s}
}
