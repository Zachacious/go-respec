// Package respec provides a fluent API for building OpenAPI specifications.
package respec

import "github.com/getkin/kin-openapi/openapi3"

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
	// securitySchemes are used to specify security requirements.
	securitySchemes []string
	// paramModifiers are used to modify parameter definitions.
	paramModifiers map[string]ParamModifier
}

// Getters for the assembler to read the final values.

// GetSummary returns the summary of the API endpoint.
func (b *Builder) GetSummary() string { return b.summary }

// GetDescription returns the description of the API endpoint.
func (b *Builder) GetDescription() string { return b.description }

// GetTags returns the tags associated with the API endpoint.
func (b *Builder) GetTags() []string { return b.tags }

// GetSecurity returns the security schemes associated with the API endpoint.
func (b *Builder) GetSecurity() []string { return b.securitySchemes }

// GetParamModifiers returns the parameter modifiers.
func (b *Builder) GetParamModifiers() map[string]ParamModifier { return b.paramModifiers }

// --- Fluent Methods ---

// NewBuilder returns a new instance of the Builder.
func NewBuilder() *Builder {
	return &Builder{
		paramModifiers: make(map[string]ParamModifier),
	}
}

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

// Security adds a security scheme to the API endpoint.
func (b *Builder) Security(schemeName string) *Builder {
	b.securitySchemes = append(b.securitySchemes, schemeName)
	return b
}

// OverrideParam overrides the definition of a parameter.
func (b *Builder) OverrideParam(name string, modifier ParamModifier) *Builder {
	b.paramModifiers[name] = modifier
	return b
}

// --- Public Marker Functions ---

// Route is a marker function for routes.
func Route(routeRegistration interface{}) *Builder {
	return NewBuilder()
}

// Group is a marker function for groups.
func Group(groupRegistration interface{}) *Builder {
	return NewBuilder()
}