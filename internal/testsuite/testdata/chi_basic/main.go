// Package main is the control fixture: the routing style respec is known to
// handle. If this one regresses, something fundamental broke.
package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Zachacious/go-respec/respec"
	"github.com/go-chi/chi/v5"
)

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type CreateUserRequest struct {
	Email string `json:"email"`
}

type APIError struct {
	Message string `json:"message"`
}

type UserHandler struct{}

// ListUsers returns every user in the system.
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	_ = r.URL.Query().Get("limit")
	json.NewEncoder(w).Encode([]User{})
}

// GetUser looks up a single user by ID.
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, _ = strconv.Atoi(id)
	json.NewEncoder(w).Encode(User{})
}

// CreateUser adds a new user.
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	json.NewDecoder(r.Body).Decode(&req)
	json.NewEncoder(w).Encode(User{})
}

func main() {
	h := &UserHandler{}
	r := chi.NewRouter()

	r.Route("/users", func(r chi.Router) {
		respec.Meta(r).Tag("Users")

		r.Get("/", respec.Handler(h.ListUsers).Summary("List all users").Unwrap())
		r.Get("/{id}", respec.Handler(h.GetUser).Summary("Get a user by ID").Unwrap())
		// A parenthesised path is still a literal. Real code contains this, and
		// failing to see through the parentheses dropped the route.
		r.Get(("/count"), respec.Handler(h.ListUsers).Summary("Count users").Unwrap())

		// A bare anonymous handler must not borrow the name of the function it
		// is registered in - that is the router constructor, not the handler.
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(User{})
		})

		r.Post("/", respec.Handler(h.CreateUser).
			Summary("Create a user").
			OperationID("createUser").
			AddResponse(http.StatusCreated, User{}).
			AddResponse(http.StatusBadRequest, APIError{}).
			Unwrap())
	})

	http.ListenAndServe(":8080", r)
}
