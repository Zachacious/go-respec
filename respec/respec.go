// Package respec provides a fluent API for adding OpenAPI metadata to http.HandlerFuncs.
package respec

import (
	"fmt"
	"go/types"
	"net/http"
	"reflect"
	"runtime"
	"sync"
)

// --- Public API ---

// DecoratedHandler is a custom type for http.HandlerFunc that allows chaining metadata methods.
type DecoratedHandler http.HandlerFunc

// Handler wraps an http.HandlerFunc, allowing metadata to be chained to it.
// This is the main entry point for developers adding endpoint-specific metadata.
// Usage: r.Post("/users", respec.Handler(myHandler).Tag("Users"))
func Handler(handler http.HandlerFunc) DecoratedHandler {
	// Use reflection to get a unique key for the handler function.
	// This name will be used as the key to store and retrieve the metadata.
	key := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()

	// Ensure a builder exists for this key, then return the handler
	// wrapped in our new type so methods can be chained.
	metadataStore.ensure(key)
	return DecoratedHandler(handler)
}

// Meta provides a way to attach metadata to a router instance within a specific scope.
// This is a marker for the static analyzer.
func Meta(router interface{}) *GroupBuilder {
	return NewGroupBuilder()
}

// --- Fluent Methods for Handlers ---

// Summary sets the summary for the operation.
func (dh DecoratedHandler) Summary(s string) DecoratedHandler {
	key := runtime.FuncForPC(reflect.ValueOf(dh).Pointer()).Name()
	if b := metadataStore.get(key); b != nil {
		b.summary = s
	}
	return dh
}

// Description sets the description for the operation.
func (dh DecoratedHandler) Description(d string) DecoratedHandler {
	key := runtime.FuncForPC(reflect.ValueOf(dh).Pointer()).Name()
	if b := metadataStore.get(key); b != nil {
		b.description = d
	}
	return dh
}

// Tag adds one or more tags to the operation.
func (dh DecoratedHandler) Tag(tags ...string) DecoratedHandler {
	key := runtime.FuncForPC(reflect.ValueOf(dh).Pointer()).Name()
	if b := metadataStore.get(key); b != nil {
		b.tags = append(b.tags, tags...)
	}
	return dh
}

// Security applies a security scheme to the operation.
func (dh DecoratedHandler) Security(schemeName string) DecoratedHandler {
	key := runtime.FuncForPC(reflect.ValueOf(dh).Pointer()).Name()
	if b := metadataStore.get(key); b != nil {
		b.security = append(b.security, schemeName)
	}
	return dh
}

// --- Builder for Groups (`respec.Meta`) ---

type GroupBuilder struct {
	tags     []string
	security []string
}

func NewGroupBuilder() *GroupBuilder {
	return &GroupBuilder{}
}
func (b *GroupBuilder) GetTags() []string     { return b.tags }
func (b *GroupBuilder) GetSecurity() []string { return b.security }
func (b *GroupBuilder) Tag(tags ...string) *GroupBuilder {
	b.tags = append(b.tags, tags...)
	return b
}
func (b *GroupBuilder) Security(schemeName string) *GroupBuilder {
	b.security = append(b.security, schemeName)
	return b
}

// --- Internal Metadata Store and Builder ---
// This holds the metadata for the static analyzer to retrieve later.

type Builder struct {
	summary     string
	description string
	tags        []string
	security    []string
}

func newBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) GetSummary() string     { return b.summary }
func (b *Builder) GetDescription() string { return b.description }
func (b *Builder) GetTags() []string      { return b.tags }
func (b *Builder) GetSecurity() []string  { return b.security }

var metadataStore = newStore()

type store struct {
	sync.RWMutex
	data map[string]*Builder
}

func newStore() *store {
	return &store{data: make(map[string]*Builder)}
}

func (s *store) ensure(key string) {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.data[key]; !ok {
		s.data[key] = newBuilder()
	}
}

func (s *store) get(key string) *Builder {
	s.RLock()
	defer s.RUnlock()
	return s.data[key]
}

// GetByHandler is called by the static analyzer.
func GetByHandler(handler types.Object) *Builder {
	key := getFuncKeyFromTypes(handler)
	if key == "" {
		return nil
	}
	return metadataStore.get(key)
}

func getFuncKeyFromTypes(obj types.Object) string {
	if obj == nil {
		return ""
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		return ""
	}
	if sig := fn.Type().(*types.Signature); sig != nil && sig.Recv() != nil {
		return fmt.Sprintf("%s.%s", sig.Recv().Type().String(), fn.Name())
	}
	if fn.Pkg() == nil {
		return ""
	}
	return fmt.Sprintf("%s.%s", fn.Pkg().Path(), fn.Name())
}
