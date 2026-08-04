// Package main exercises the v1 builder API: the option-based parameter and
// response forms, Hide, Public, and response headers with schemas.
//
// This fixture is the contract test for the public API. Anything that changes
// its golden output is a change users will feel.
package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Zachacious/go-respec/respec"
	"github.com/go-chi/chi/v5"
)

type Session struct {
	Token     string     `json:"token"`
	IssuedAt  time.Time  `json:"issued_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Scopes    []string   `json:"scopes"`
	Raw       []byte     `json:"raw,omitempty"`
}

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type APIError struct {
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

// Login exchanges credentials for a session.
func Login(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	json.NewDecoder(r.Body).Decode(&creds)
	json.NewEncoder(w).Encode(Session{})
}

// ListSessions returns the caller's active sessions.
//
// Results are ordered by creation time.
// @tags Sessions
func ListSessions(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode([]Session{})
}

// DebugDump exposes internal state and must not appear in the spec.
func DebugDump(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{})
}

func main() {
	r := chi.NewRouter()

	r.Route("/auth", func(r chi.Router) {
		respec.Meta(r).Tag("Auth").Security("BearerAuth")

		// Public opts this route out of the group's security requirement.
		r.Post("/login", respec.Handler(Login).
			Summary("Log in").
			Public().
			RequestBody(Credentials{}).
			Response(http.StatusOK, respec.Schema(Session{}), respec.Desc("Session issued")).
			Response(http.StatusUnauthorized, respec.Schema(APIError{}), respec.Desc("Bad credentials")).
			ResponseHeader(http.StatusOK, "X-Rate-Limit", respec.HeaderDoc("Requests remaining"), respec.HeaderType("integer")).
			Unwrap())

		r.Get("/sessions", respec.Handler(ListSessions).
			Summary("List sessions").
			QueryParam("limit", respec.Type("integer"), respec.Doc("Maximum results"), respec.Example("25")).
			QueryParam("active", respec.Type("boolean"), respec.Doc("Only active sessions")).
			HeaderParam("X-Request-Id", respec.Doc("Correlation id"), respec.Required()).
			Response(http.StatusOK, respec.Schema([]Session{})).
			Unwrap())

		// Hidden from the published spec.
		r.Get("/debug", respec.Handler(DebugDump).
			Summary("Dump internal state").
			Hide().
			Unwrap())

		// A whole subgroup opting out of the group's security. Tags still come
		// down from the parent; only security is cleared.
		r.Route("/status", func(r chi.Router) {
			respec.Meta(r).Public()
			r.Get("/ping", respec.Handler(Login).Summary("Ping").Unwrap())
		})
	})

	http.ListenAndServe(":8080", r)
}
