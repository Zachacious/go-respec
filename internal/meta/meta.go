// Package meta holds the structures the analyzer fills in while reading a
// respec builder chain out of source code.
//
// These types used to live in the public respec package. They describe results
// of static analysis, not anything a program does at run time, and two of them
// carry go/ast expressions - which meant every project that imported respec to
// annotate its handlers also compiled go/ast, go/scanner, and go/token into its
// binary. Keeping them internal also means they are not part of the v1 API
// promise, so the analyzer stays free to change them.
package meta

import "go/ast"

// ParameterOverride is an explicitly declared operation parameter.
type ParameterOverride struct {
	In          string
	Name        string
	Description string
	Required    bool
	Deprecated  bool
	// Example is rendered as the parameter's example value.
	Example string
	// Schema names a primitive type: string, integer, number, or boolean.
	Schema string
}

// ResponseHeaderOverride is a header declared on a specific response code.
type ResponseHeaderOverride struct {
	Code        int
	Name        string
	Description string
	// Schema names a primitive type. A header without one is rejected by
	// strict validators.
	Schema string
}

// ResponseOverride is an explicitly declared response.
type ResponseOverride struct {
	Code        int
	Description string
	// ContentExpr is the expression whose type becomes the response schema.
	// Nil when the response carries only a description.
	ContentExpr ast.Expr
}

// ServerOverride is an operation-level server entry.
type ServerOverride struct {
	URL         string
	Description string
}

// ExternalDocsOverride is an operation-level external documentation link.
type ExternalDocsOverride struct {
	URL         string
	Description string
}

// HandlerMetadata is everything parsed from one respec.Handler chain.
type HandlerMetadata struct {
	Summary         string
	Description     string
	Tags            []string
	Security        []string
	RequestBodyExpr ast.Expr
	Responses       []ResponseOverride
	Parameters      []ParameterOverride
	ResponseHeaders []ResponseHeaderOverride
	Servers         []ServerOverride
	ExternalDocs    *ExternalDocsOverride
	OperationID     string
	Deprecated      bool
	Extensions      map[string]any
	// Hidden omits the operation from the generated spec entirely.
	Hidden bool
	// NoSecurity clears any inherited security requirement, which is how an
	// endpoint inside an authenticated group is marked public.
	NoSecurity bool
}

// GroupMetadata is everything parsed from one respec.Meta chain. It applies to
// every route registered on that routing scope.
type GroupMetadata struct {
	Tags       []string
	Security   []string
	Deprecated bool
	NoSecurity bool
}
