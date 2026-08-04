// Package main covers github.com/Zachacious/go-respec issue #1: a respec.Handler
// chain that is not written inline in the route registration call.
//
// Every route in this fixture SHOULD appear in the generated spec with its
// metadata attached. Before the Phase 3 fix, most of them vanished silently.
package main

import (
	"encoding/json"
	"net/http"

	"github.com/Zachacious/go-respec/respec"
	"github.com/go-chi/chi/v5"
)

type Address struct {
	ID     string `json:"id"`
	Street string `json:"street"`
}

type JSONError struct {
	Message string `json:"message"`
}

type AddressHandler struct{}

// CreateAddress stores a new address.
func (h *AddressHandler) CreateAddress(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(Address{})
}

// GetAddress fetches one address.
func (h *AddressHandler) GetAddress(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(Address{})
}

// DeleteAddress removes an address.
func (h *AddressHandler) DeleteAddress(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// CreateAddressSpec wraps the builder in a factory method - the exact shape
// reported in issue #1.
func (h *AddressHandler) CreateAddressSpec() http.HandlerFunc {
	return respec.Handler(h.CreateAddress).
		Summary("Create a new address").
		AddResponse(http.StatusCreated, Address{}).
		AddResponse(http.StatusBadRequest, JSONError{}).
		Unwrap()
}

// healthSpec wraps an anonymous function - the shape reported in the issue's
// follow-up comment about echo.
func healthSpec() http.HandlerFunc {
	return respec.Handler(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(JSONError{})
	}).
		Summary("Health check").
		Unwrap()
}

func main() {
	h := &AddressHandler{}
	r := chi.NewRouter()

	r.Route("/addresses", func(r chi.Router) {
		respec.Meta(r).Tag("Addresses")

		// Indirect: factory method call.
		r.Post("/", h.CreateAddressSpec())

		// Indirect: factory returning a wrapped anonymous function.
		r.Get("/health", healthSpec())

		// Indirect: builder result held in a local variable.
		getAddress := respec.Handler(h.GetAddress).
			Summary("Get an address").
			Unwrap()
		r.Get("/{id}", getAddress)

		// Control: inline, the style that already worked.
		r.Delete("/{id}", respec.Handler(h.DeleteAddress).
			Summary("Delete an address").
			Unwrap())
	})

	http.ListenAndServe(":8080", r)
}
