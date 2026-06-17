package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wengti0608/golang-docker-postgres/store"
)

// createUserRequest is the expected JSON body for creating a user. The binding
// tags make Gin validate the input: name is required, email is required and
// must look like an email address.
type createUserRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

// CreateUser handles POST /users: validate the body, insert via the store, and
// return the created user.
func (h *Handler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.store.CreateUser(c.Request.Context(), req.Name, req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create user"})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// ListUsers handles GET /users: return all users.
func (h *Handler) ListUsers(c *gin.Context) {
	users, err := h.store.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list users"})
		return
	}

	c.JSON(http.StatusOK, users)
}

// DeleteUser handles DELETE /users/:id: remove the user with the given id.
func (h *Handler) DeleteUser(c *gin.Context) {
	// Path params arrive as strings; the id column is an integer, so convert
	// and reject anything that is not a valid number.
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	if err := h.store.DeleteUser(c.Request.Context(), id); err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete user"})
		return
	}

	// 204 No Content: success, with no body to return.
	c.Status(http.StatusNoContent)
}
