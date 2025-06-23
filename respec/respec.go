// Package respec provides a fluent API for adding OpenAPI metadata to handlers.
package respec

// --- Public API for Handler-Level Metadata ---

// HandlerBuilder is a generic struct that holds a handler of any type `T`
// and the chainable metadata associated with it.
type HandlerBuilder[T any] struct {
	handler     T
	summary     string
	description string
	tags        []string
	security    []string
}

// Handler is a generic function. It accepts a handler of any type `T` and
// returns a builder specialized for that type, `*HandlerBuilder[T]`.
func Handler[T any](handler T) *HandlerBuilder[T] {
	return &HandlerBuilder[T]{
		handler: handler,
	}
}

// Unwrap returns the original handler, preserving its exact original type `T`.
// This makes the entire chain compatible with any Go web framework.
func (hb *HandlerBuilder[T]) Unwrap() T {
	return hb.handler
}

// Summary sets the summary for the operation.
func (hb *HandlerBuilder[T]) Summary(s string) *HandlerBuilder[T] {
	hb.summary = s
	return hb
}

// Description sets the description for the operation.
func (hb *HandlerBuilder[T]) Description(d string) *HandlerBuilder[T] {
	hb.description = d
	return hb
}

// Tag adds one or more tags to the operation.
func (hb *HandlerBuilder[T]) Tag(tags ...string) *HandlerBuilder[T] {
	hb.tags = append(hb.tags, tags...)
	return hb
}

// Security applies a security scheme to the operation.
func (hb *HandlerBuilder[T]) Security(schemeName string) *HandlerBuilder[T] {
	hb.security = append(hb.security, schemeName)
	return hb
}

// --- Public API for Group-Level Metadata ---

// GroupBuilder stores metadata for a group of routes.
type GroupBuilder struct {
	tags     []string
	security []string
}

// NewGroupBuilder creates a new GroupBuilder instance.
func NewGroupBuilder() *GroupBuilder {
	return &GroupBuilder{}
}

// Meta provides a way to attach metadata to a router instance within a specific scope.
// This is a marker for the static analyzer.
func Meta(router interface{}) *GroupBuilder {
	// Note: The 'router' argument is a placeholder for the static analyzer
	// to identify the variable associated with this metadata. It is not used at runtime.
	return NewGroupBuilder()
}

// GetTags returns the tags for the group.
func (b *GroupBuilder) GetTags() []string { return b.tags }

// GetSecurity returns the security schemes for the group.
func (b *GroupBuilder) GetSecurity() []string { return b.security }

// Tag adds tags to the group.
func (b *GroupBuilder) Tag(tags ...string) *GroupBuilder {
	b.tags = append(b.tags, tags...)
	return b
}

// Security adds a security scheme to the group.
func (b *GroupBuilder) Security(schemeName string) *GroupBuilder {
	b.security = append(b.security, schemeName)
	return b
}

// --- Internal Metadata Structures ---
// These are used by the analyzer after parsing the AST.

// HandlerMetadata holds the parsed metadata for a single operation.
type HandlerMetadata struct {
	Summary     string
	Description string
	Tags        []string
	Security    []string
}
