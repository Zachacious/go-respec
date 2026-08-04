// Package main covers same-named types in different packages.
//
// admin.User and public.User must produce two distinct components. Collapsing
// them means the public endpoint documents admin-only fields, which is a
// disclosure bug, not just a cosmetic one.
package main

import (
	"encoding/json"
	"net/http"

	"github.com/Zachacious/go-respec/respec"
	"github.com/go-chi/chi/v5"

	"respectest/schema_collision/admin"
	internalpkg "respectest/schema_collision/internal"
	"respectest/schema_collision/public"
)

// AdminGetUser returns the full user record.
func AdminGetUser(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(admin.User{})
}

// PublicGetUser returns the publicly visible user record.
func PublicGetUser(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(public.User{})
}

// InternalGetUser returns the internal moderation view.
func InternalGetUser(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(internalpkg.User{})
}

func main() {
	r := chi.NewRouter()
	r.Get("/admin/user", respec.Handler(AdminGetUser).Summary("Admin view of a user").Unwrap())
	r.Get("/public/user", respec.Handler(PublicGetUser).Summary("Public view of a user").Unwrap())
	r.Get("/internal/user", respec.Handler(InternalGetUser).Summary("Internal view of a user").Unwrap())
	http.ListenAndServe(":8080", r)
}
