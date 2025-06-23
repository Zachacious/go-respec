// Package respec provides a fluent API for adding OpenAPI metadata to http.HandlerFuncs.
package respec

import (
	"github.com/getkin/kin-openapi/openapi3"
)

// --- Public API ---

// RouteBuilder is a generic struct that can hold a handler of any type `T`,
// along with its associated metadata.
type RouteBuilder[T any] struct {
	handler        T
	summary        string
	description    string
	tags           []string
	security       []string
	overrideParams map[string]func(*openapi3.Parameter)
}

// Route is a generic function. It accepts a handler of any type `T` and
// returns a builder specialized for that type, `*RouteBuilder[T]`.
func Route[T any](handler T) *RouteBuilder[T] {
	return &RouteBuilder[T]{
		handler:        handler,
		overrideParams: make(map[string]func(*openapi3.Parameter)),
	}
}

// Unwrap returns the original handler, preserving its exact original type `T`.
func (rb *RouteBuilder[T]) Unwrap() T {
	return rb.handler
}

// --- Fluent Methods for Handlers ---

// Summary sets the summary for the operation.
func (rb *RouteBuilder[T]) Summary(s string) *RouteBuilder[T] {
	rb.summary = s
	return rb
}

// Description sets the description for the operation.
func (rb *RouteBuilder[T]) Description(d string) *RouteBuilder[T] {
	rb.description = d
	return rb
}

// Tag adds one or more tags to the operation.
func (rb *RouteBuilder[T]) Tag(tags ...string) *RouteBuilder[T] {
	rb.tags = append(rb.tags, tags...)
	return rb
}

// Security applies a security scheme to the operation.
func (rb *RouteBuilder[T]) Security(schemeName string) *RouteBuilder[T] {
	rb.security = append(rb.security, schemeName)
	return rb
}

// OverrideParam provides fine-grained control over a single parameter.
func (rb *RouteBuilder[T]) OverrideParam(name string, modifier func(*openapi3.Parameter)) *RouteBuilder[T] {
	rb.overrideParams[name] = modifier
	return rb
}

// --- Builder for Groups (`respec.Meta`) ---
// This section remains unchanged as requested.

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

// --- Internal Metadata Structures ---
// These are used by the analyzer after parsing the AST.

// BuilderMetadata holds the parsed metadata for a single operation.
type BuilderMetadata struct {
	Summary        string
	Description    string
	Tags           []string
	Security       []string
	OverrideParams map[string]func(*openapi3.Parameter)
}

// GetByHandler is no longer used and has been removed.
// The analyzer will now populate BuilderMetadata directly.

// We keep getFuncKeyFromTypes as it might be useful for other parts of the analyzer,
// though it is not directly used by the new builder pattern.
// func getFuncKeyFromTypes(obj types.Object) string {
// 	if obj == nil {
// 		return ""
// 	}
// 	fn, ok := obj.(*types.Func)
// 	if !ok {
// 		return ""
// 	}
// 	// This part of the logic seems to have a bug in the original code,
// 	// `sig.Recv().Type().String()` might not be what you want.
// 	// A fully qualified path is usually better.
// 	// For now, we will keep it as is.
// 	if sig := fn.Type().(*types.Signature); sig != nil && sig.Recv() != nil {
// 		// Example: (*github.com/me/project/handlers.UserHandlers).CreateUser
// 		return fn.FullName()
// 	}
// 	if fn.Pkg() == nil {
// 		return ""
// 	}
// 	// Example: github.com/me/project/handlers.CreateUser
// 	return fn.FullName()
// }
