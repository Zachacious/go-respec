// Package main covers the Go 1.22+ net/http router, which needs no third-party
// dependency at all.
//
// ServeMux folds the HTTP method into the pattern string, so this fixture is
// what proves the endpointPatterns config axis works: nothing in the analyzer
// knows what a ServeMux is.
package main

import (
	"encoding/json"
	"net/http"

	"github.com/Zachacious/go-respec/respec"
)

type Book struct {
	ISBN   string `json:"isbn"`
	Title  string `json:"title"`
	Author string `json:"author,omitempty"`
}

type NewBookRequest struct {
	Title string `json:"title"`
}

// ListBooks returns the catalogue.
func ListBooks(w http.ResponseWriter, r *http.Request) {
	_ = r.URL.Query().Get("author")
	json.NewEncoder(w).Encode([]Book{})
}

// GetBook returns one book by ISBN.
func GetBook(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(Book{})
}

// CreateBook adds a book to the catalogue.
func CreateBook(w http.ResponseWriter, r *http.Request) {
	var req NewBookRequest
	json.NewDecoder(r.Body).Decode(&req)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(Book{})
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /books", respec.Handler(ListBooks).Summary("List books").Unwrap())
	mux.HandleFunc("GET /books/{isbn}", respec.Handler(GetBook).Summary("Get a book").Unwrap())
	mux.HandleFunc("POST /books", respec.Handler(CreateBook).
		Summary("Create a book").
		AddResponse(http.StatusCreated, Book{}).
		Unwrap())

	// A pattern with no method matches every verb. Documenting it five times
	// would be noise, so it is recorded once as GET.
	mux.HandleFunc("/legacy", respec.Handler(ListBooks).Summary("Legacy endpoint").Unwrap())

	// Static assets. The handler is not a function respec can read, and the
	// trailing slash is a subtree match that must survive into the spec.
	mux.Handle("/static/", http.FileServer(http.Dir("./public")))

	http.ListenAndServe(":8080", mux)
}
