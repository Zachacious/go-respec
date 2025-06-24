package respec

import (
	"go/ast"
)

// HandlerBuilder is a generic struct that holds a handler of any type `T`
// and the chainable metadata associated with it.
type HandlerBuilder[T any] struct {
	handler     T
	summary     string
	description string
	tags        []string
	security    []string
	requestBody any
	responses   map[int]any
	operationID string
	deprecated  bool
}

// Handler is a generic function. It accepts a handler of any type `T` and
// returns a builder specialized for that type, `*HandlerBuilder[T]`.
func Handler[T any](handler T) *HandlerBuilder[T] {
	return &HandlerBuilder[T]{
		handler:   handler,
		responses: make(map[int]any),
	}
}

// Unwrap returns the original handler, preserving its exact original type `T`.
func (hb *HandlerBuilder[T]) Unwrap() T                               { return hb.handler }
func (hb *HandlerBuilder[T]) Summary(s string) *HandlerBuilder[T]     { hb.summary = s; return hb }
func (hb *HandlerBuilder[T]) Description(d string) *HandlerBuilder[T] { hb.description = d; return hb }
func (hb *HandlerBuilder[T]) Tag(tags ...string) *HandlerBuilder[T] {
	hb.tags = append(hb.tags, tags...)
	return hb
}
func (hb *HandlerBuilder[T]) Security(schemeName string) *HandlerBuilder[T] {
	hb.security = append(hb.security, schemeName)
	return hb
}
func (hb *HandlerBuilder[T]) RequestBody(obj any) *HandlerBuilder[T] { hb.requestBody = obj; return hb }
func (hb *HandlerBuilder[T]) AddResponse(code int, obj any) *HandlerBuilder[T] {
	hb.responses[code] = obj
	return hb
}
func (hb *HandlerBuilder[T]) OperationID(id string) *HandlerBuilder[T] {
	hb.operationID = id
	return hb
}
func (hb *HandlerBuilder[T]) Deprecate(d bool) *HandlerBuilder[T] { hb.deprecated = d; return hb }

// GroupBuilder stores metadata for a group of routes.
type GroupBuilder struct {
	tags       []string
	security   []string
	deprecated bool
}

// NewGroupBuilder creates a new GroupBuilder instance.
func NewGroupBuilder() *GroupBuilder                     { return &GroupBuilder{} }
func (b *GroupBuilder) GetTags() []string                { return b.tags }
func (b *GroupBuilder) GetSecurity() []string            { return b.security }
func (b *GroupBuilder) Tag(tags ...string) *GroupBuilder { b.tags = append(b.tags, tags...); return b }
func (b *GroupBuilder) Security(schemeName string) *GroupBuilder {
	b.security = append(b.security, schemeName)
	return b
}
func (b *GroupBuilder) Deprecate(d bool) *GroupBuilder { b.deprecated = d; return b }
func Meta(router interface{}) *GroupBuilder            { return NewGroupBuilder() }

// --- Internal Metadata Structures ---

// HandlerMetadata holds the parsed metadata for a single operation.
type HandlerMetadata struct {
	Summary         string
	Description     string
	Tags            []string
	Security        []string
	RequestBodyExpr ast.Expr
	ResponseExprs   map[int]ast.Expr
	OperationID     string
	Deprecated      bool
}
