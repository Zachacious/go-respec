// Package main covers response-code inference and security scheme round-tripping.
//
// Two things this fixture pins down:
//   - A handler that calls WriteHeader(201) then Encode(x) must document 201
//     only. It previously also emitted a spurious 200 because Encoder.Encode is
//     matched by both the hardcoded branch and the default config pattern.
//   - The apiKey scheme in .respec.yaml must keep its "in" and "name". Dropping
//     them produces a spec that respec's own validate command rejects.
package main

import (
	"encoding/json"
	"net/http"

	"github.com/Zachacious/go-respec/respec"
	"github.com/go-chi/chi/v5"
)

type Widget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CreateWidget makes a widget and reports 201.
func CreateWidget(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(Widget{})
}

// GetWidget returns a widget with a plain 200.
func GetWidget(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(Widget{})
}

// DeleteWidget reports 204 and writes no body.
func DeleteWidget(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	r := chi.NewRouter()
	r.Post("/widgets", respec.Handler(CreateWidget).Summary("Create a widget").Unwrap())
	r.Get("/widgets/{id}", respec.Handler(GetWidget).Summary("Get a widget").Unwrap())
	r.Delete("/widgets/{id}", respec.Handler(DeleteWidget).Summary("Delete a widget").Unwrap())
	http.ListenAndServe(":8080", r)
}
