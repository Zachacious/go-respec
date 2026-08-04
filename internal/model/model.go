package model

import (
	"go/ast"
	"go/types"

	"github.com/Zachacious/go-respec/internal/meta"
	"github.com/getkin/kin-openapi/openapi3"
)

// GroupMetadataMap is defined here to be shared across packages.
type GroupMetadataMap map[types.Object]*meta.GroupMetadata

// APIModel is the top-level container for the entire discovered API.
type APIModel struct {
	openapi3.T
	// RouteGraph represents the routing tree of the API.
	RouteGraph *RouteNode
	// GroupMetadata holds metadata from .Meta() calls.
	GroupMetadata GroupMetadataMap
	// Diagnostics reports everything the analyzer could not fully resolve.
	Diagnostics []Diagnostic
}

// Diagnostic is a message about the analysis, anchored to a source location.
type Diagnostic struct {
	// Severity is "warning" or "info".
	Severity string
	// Location is "file:line:col", empty if the position could not be resolved.
	Location string
	// Message describes what respec could not do, and what to check.
	Message string
}

// RouteNode represents a single routing scope (a router or a group).
type RouteNode struct {
	// GoVar holds a reference to the Go variable for this router/group.
	GoVar types.Object
	// PathPrefix is the path prefix of the current routing scope.
	PathPrefix string
	// Parent is the parent node in the routing tree.
	Parent *RouteNode
	// Children are the child nodes in the routing tree.
	Children []*RouteNode
	// Operations are the API endpoints in the current routing scope.
	Operations []*Operation
	// InferredSecurity holds the names of security schemes inferred from middleware.
	InferredSecurity []string
	// Tags holds tags from .Meta() calls for hierarchical application.
	Tags []string
	// Deprecated marks whether this entire node and its children are deprecated.
	Deprecated bool
	// NoSecurity marks this scope, and everything under it, as exempt from
	// security inherited from enclosing scopes.
	NoSecurity bool
}

// Operation represents a single API endpoint (e.g., GET /users/{id}).
type Operation struct {
	// HTTPMethod is the HTTP method of the API endpoint (e.g., GET, POST, PUT, DELETE).
	HTTPMethod string
	// RoutePath is the path exactly as written in the registration call, before
	// any parent prefixes are applied and before framework-specific parameter
	// syntax is normalised.
	RoutePath string
	// FullPath is the full path of the API endpoint. It is derived from
	// RoutePath plus the prefixes of every ancestor node, and is only valid
	// after the analyzer's finalize pass has run. Routers can be re-parented
	// during analysis (see Mount), so this cannot be computed at discovery time.
	FullPath string
	// Node is the routing scope this operation was registered on. The chain of
	// parents from here supplies the path prefixes, tags, and security.
	Node *RouteNode
	// HandlerBody is the body of the resolved handler. This is what gets scanned
	// for request/response schemas and parameters. It comes either from a named
	// function declaration or from a function literal passed to respec.Handler.
	HandlerBody *ast.BlockStmt
	// HandlerPackage is the package name of the handler function.
	HandlerPackage string
	// HandlerName is the name of the handler function.
	HandlerName string
	// GoHandler holds a reference to the Go handler function.
	GoHandler types.Object
	// HandlerMetadata holds metadata from the fluent builder.
	HandlerMetadata *meta.HandlerMetadata
	// DocTags holds tags from an @tags line in the handler's doc comment. They
	// sit between inherited group tags and an explicit .Tag() override, so they
	// are kept separate rather than written straight onto the spec.
	DocTags []string
	// Spec is the OpenAPI specification of the API endpoint.
	Spec *openapi3.Operation
}
