package model

import (
	"go/types"

	"github.com/Zachacious/go-respec/respec"
	"github.com/getkin/kin-openapi/openapi3"
)

// GroupMetadataMap is defined here to be shared across packages.
type GroupMetadataMap map[types.Object]*respec.GroupBuilder

// APIModel is the top-level container for the entire discovered API.
type APIModel struct {
	openapi3.T
	// RouteGraph represents the routing tree of the API.
	RouteGraph *RouteNode
	// GroupMetadata holds metadata from .Meta() calls.
	GroupMetadata GroupMetadataMap
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
	Deprecated bool // <-- ADDED
}

// Operation represents a single API endpoint (e.g., GET /users/{id}).
type Operation struct {
	// HTTPMethod is the HTTP method of the API endpoint (e.g., GET, POST, PUT, DELETE).
	HTTPMethod string
	// FullPath is the full path of the API endpoint.
	FullPath string
	// HandlerPackage is the package name of the handler function.
	HandlerPackage string
	// HandlerName is the name of the handler function.
	HandlerName string
	// GoHandler holds a reference to the Go handler function.
	GoHandler types.Object
	// HandlerMetadata holds metadata from the fluent builder.
	HandlerMetadata *respec.HandlerMetadata
	// Spec is the OpenAPI specification of the API endpoint.
	Spec *openapi3.Operation
}
