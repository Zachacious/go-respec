// Package respec provides a fluent API for adding OpenAPI metadata to http.HandlerFuncs.
package respec

import (
	"fmt"
	"go/types"
	"net/http"
	"reflect"
	"runtime"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
)

// --- Public API ---

// H wraps an http.HandlerFunc, allowing metadata to be chained to it.
// This is the main entry point for developers using the respec library.
// Usage: r.Post("/users", respec.H(myHandler).Tag("Users"))
func H(handler http.HandlerFunc) *Builder {
	builder := newBuilder()

	// Use reflection to get the unique, fully-qualified name of the handler function.
	// This name will be used as the key to store and retrieve the metadata.
	key := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	metadataStore.set(key, builder)

	// The builder itself implements http.Handler, so it can be passed to router methods.
	builder.handler = handler
	return builder
}

// --- Types and Builder ---

// Modifier functions for granular control.
type ParamModifier func(p *openapi3.Parameter)
type ResponseModifier func(r *openapi3.Response)

// Builder holds the metadata provided by the developer through the fluent API.
type Builder struct {
	// summary is a brief summary of the API endpoint.
	summary string
	// description is a longer description of the API endpoint.
	description string
	// tags are used to group API endpoints.
	tags []string
	// security are the names of security schemes to apply.
	security []string
	// paramModifiers are used to modify parameter definitions.
	paramModifiers map[string]ParamModifier
	// handler is the original http.HandlerFunc being wrapped.
	handler http.HandlerFunc
}

// newBuilder returns a new instance of the Builder. It is unexported because
// the public API uses H() as the entry point.
func newBuilder() *Builder {
	return &Builder{
		paramModifiers: make(map[string]ParamModifier),
	}
}

// ServeHTTP allows the Builder to satisfy the http.Handler interface.
// It simply calls the original handler, making the wrapper transparent at runtime.
func (b *Builder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if b.handler != nil {
		b.handler(w, r)
	}
}

// Getters for the static analyzer to read the final values.

// GetSummary returns the summary of the API endpoint.
func (b *Builder) GetSummary() string { return b.summary }

// GetDescription returns the description of the API endpoint.
func (b *Builder) GetDescription() string { return b.description }

// GetTags returns the tags associated with the API endpoint.
func (b *Builder) GetTags() []string { return b.tags }

// GetSecurity returns the security schemes associated with the API endpoint.
func (b *Builder) GetSecurity() []string { return b.security }

// GetParamModifiers returns the parameter modifiers.
func (b *Builder) GetParamModifiers() map[string]ParamModifier { return b.paramModifiers }

// --- Fluent Methods ---

// Summary sets the summary of the API endpoint.
func (b *Builder) Summary(s string) *Builder {
	b.summary = s
	return b
}

// Description sets the description of the API endpoint.
func (b *Builder) Description(d string) *Builder {
	b.description = d
	return b
}

// Tag adds one or more tags to the API endpoint.
func (b *Builder) Tag(tags ...string) *Builder {
	b.tags = append(b.tags, tags...)
	return b
}

// Security adds a security scheme to the API endpoint. The name must match
// a key in the `securitySchemes` section of `.respec.yaml`.
func (b *Builder) Security(schemeName string) *Builder {
	b.security = append(b.security, schemeName)
	return b
}

// OverrideParam provides a function to modify an inferred parameter.
// This is an advanced feature to gain fine-grained control over a parameter's definition.
func (b *Builder) OverrideParam(name string, modifier ParamModifier) *Builder {
	b.paramModifiers[name] = modifier
	return b
}

// --- Internal Metadata Store ---
// This package-level variable holds the metadata, acting as a bridge between
// the developer's running code and the static analysis tool.

var metadataStore = newStore()

type store struct {
	sync.RWMutex
	data map[string]*Builder
}

func newStore() *store {
	return &store{
		data: make(map[string]*Builder),
	}
}

func (s *store) set(key string, b *Builder) {
	s.Lock()
	defer s.Unlock()
	s.data[key] = b
}

// GetByHandler is called by the static analyzer. It constructs a key from the
// analyzer's type information and uses it to look up the stored metadata.
func GetByHandler(handler types.Object) *Builder {
	key := getFuncKeyFromTypes(handler)
	if key == "" {
		return nil
	}

	metadataStore.RLock()
	defer metadataStore.RUnlock()
	return metadataStore.data[key]
}

// getFuncKeyFromTypes creates a unique string identifier from a types.Object,
// matching the format produced by the runtime reflection.
func getFuncKeyFromTypes(obj types.Object) string {
	if obj == nil {
		return ""
	}

	fn, ok := obj.(*types.Func)
	if !ok {
		return ""
	}

	// For methods, the full name includes the receiver type.
	if sig := fn.Type().(*types.Signature); sig != nil && sig.Recv() != nil {
		// The receiver's type string is fully-qualified, e.g., "github.com/user/project.(*MyType)"
		// We need to clean it up to match the runtime's output.
		recvStr := sig.Recv().Type().String()
		return fmt.Sprintf("%s.%s", recvStr, fn.Name())
	}

	// For regular functions.
	if fn.Pkg() == nil {
		return ""
	}
	return fmt.Sprintf("%s.%s", fn.Pkg().Path(), fn.Name())
}
