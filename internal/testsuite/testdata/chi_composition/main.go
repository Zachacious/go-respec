// Package main covers router composition: registering routes from a helper
// function and mounting a sub-router. Both are ordinary chi idioms and both
// depend on a tracked router value surviving a function boundary.
//
// Expected paths: /users, /users/{id}, /health/, /ok
// Before the Phase 1 dataflow fix the helper routes vanished and the mounted
// sub-router's route landed at "/" instead of "/health/".
package main

import (
	"encoding/json"
	"net/http"

	"github.com/Zachacious/go-respec/respec"
	"github.com/go-chi/chi/v5"
)

type User struct {
	ID string `json:"id"`
}

type Status struct {
	OK bool `json:"ok"`
}

// ListUsers returns all users.
func ListUsers(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode([]User{}) }

// GetUser returns one user.
func GetUser(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(User{}) }

// Health reports service health.
func Health(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(Status{}) }

// Ping is a liveness probe.
func Ping(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(Status{}) }

// RegisterUserRoutes attaches user routes to an existing router. The tracked
// router value has to survive being passed as a parameter.
func RegisterUserRoutes(r chi.Router) {
	r.Get("/users", respec.Handler(ListUsers).Summary("List users").Unwrap())
	r.Get("/users/{id}", respec.Handler(GetUser).Summary("Get a user").Unwrap())
}

// healthRouter builds a sub-router. The tracked value has to survive a return.
func healthRouter() chi.Router {
	r := chi.NewRouter()
	r.Get("/", respec.Handler(Health).Summary("Health check").Unwrap())
	return r
}

func main() {
	r := chi.NewRouter()

	RegisterUserRoutes(r)
	r.Mount("/health", healthRouter())

	// Control: direct registration on the root router.
	r.Get("/ok", respec.Handler(Ping).Summary("Ping").Unwrap())

	http.ListenAndServe(":8080", r)
}
