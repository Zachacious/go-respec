package model

import (
	"go/types"

	"github.com/Zachacious/go-respec/internal/builder"
	"github.com/getkin/kin-openapi/openapi3"
)

// APIModel is the top-level container for the entire discovered API.
type APIModel struct {
	openapi3.T
	RouteGraph *RouteNode
}

// RouteNode represents a single routing scope (a router or a group).
type RouteNode struct {
	// ADDED: A reference to the Go variable for this router/group.
	GoVar types.Object

	PathPrefix string
	Parent     *RouteNode
	Children   []*RouteNode
	Operations []*Operation
}

// Operation represents a single API endpoint (e.g., GET /users/{id}).
type Operation struct {
	HTTPMethod string
	FullPath   string

	HandlerPackage string
	HandlerName    string
	GoHandler      types.Object

	// ADDED: To hold metadata from the fluent builder
	BuilderMetadata *builder.Builder

	Spec *openapi3.Operation
}
