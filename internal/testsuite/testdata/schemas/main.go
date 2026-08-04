// Package main pins schema generation for the Go type shapes that are easy to
// get wrong: recursion, embedding, generics, aliases, and the standard types
// whose JSON form has nothing to do with their Go structure.
package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Zachacious/go-respec/respec"
	"github.com/go-chi/chi/v5"
)

// Base is embedded, so its fields belong to the embedding object rather than to
// a nested one, matching how encoding/json treats it.
type Base struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

// Node refers to itself both directly and through a slice and a map.
type Node struct {
	Base
	Name     string           `json:"name"`
	Children []*Node          `json:"children,omitempty"`
	Parent   *Node            `json:"parent,omitempty"`
	Meta     map[string]any   `json:"meta,omitempty"`
	Peers    map[string]*Node `json:"peers,omitempty"`
	Blob     []byte           `json:"blob,omitempty"`
	Ratio    float64          `json:"ratio"`
	Active   bool             `json:"active"`
	// No omitempty, so a nil pointer really is encoded as null: this field is
	// always present and genuinely nullable.
	Owner    *Base   `json:"owner"`
	Nickname *string `json:"nickname"`
	Secret   string  `json:"-"`
	internal string
}

// Left and Right are mutually recursive.
type Left struct {
	Right *Right `json:"right,omitempty"`
}

type Right struct {
	Left *Left `json:"left,omitempty"`
}

// Envelope is generic. Each instantiation needs its own component, or one
// silently overwrites the other.
type Envelope[T any] struct {
	Data  T      `json:"data"`
	Error string `json:"error,omitempty"`
}

// GetNode returns a tree node.
func GetNode(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(Node{}) }

// GetLeft returns a mutually recursive value.
func GetLeft(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(Left{}) }

// GetEnvelopedNode returns a node in an envelope.
func GetEnvelopedNode(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(Envelope[Node]{})
}

// GetEnvelopedLeft returns a Left in the same envelope type.
func GetEnvelopedLeft(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(Envelope[Left]{})
}

func main() {
	r := chi.NewRouter()
	r.Get("/nodes", respec.Handler(GetNode).Unwrap())
	r.Get("/left", respec.Handler(GetLeft).Unwrap())
	r.Get("/enveloped/node", respec.Handler(GetEnvelopedNode).Unwrap())
	r.Get("/enveloped/left", respec.Handler(GetEnvelopedLeft).Unwrap())
	http.ListenAndServe(":8080", r)
}
