// Package builder contains the source for the user-facing respec library.
package respec

import "github.com/getkin/kin-openapi/openapi3"

// Modifier functions for granular control.
type ParamModifier func(p *openapi3.Parameter)
type ResponseModifier func(r *openapi3.Response)

// Builder holds the metadata provided by the developer through the fluent API.
type Builder struct {
	summary         string
	description     string
	tags            []string
	securitySchemes []string
	paramModifiers  map[string]ParamModifier
}

// Getters for the assembler to read the final values
func (b *Builder) GetSummary() string                          { return b.summary }
func (b *Builder) GetDescription() string                      { return b.description }
func (b *Builder) GetTags() []string                           { return b.tags }
func (b *Builder) GetSecurity() []string                       { return b.securitySchemes }
func (b *Builder) GetParamModifiers() map[string]ParamModifier { return b.paramModifiers }

// --- Fluent Methods ---

func NewBuilder() *Builder {
	return &Builder{
		paramModifiers: make(map[string]ParamModifier),
	}
}

func (b *Builder) Summary(s string) *Builder {
	b.summary = s
	return b
}

func (b *Builder) Description(d string) *Builder {
	b.description = d
	return b
}

func (b *Builder) Tag(tags ...string) *Builder {
	b.tags = append(b.tags, tags...)
	return b
}

func (b *Builder) Security(schemeName string) *Builder {
	b.securitySchemes = append(b.securitySchemes, schemeName)
	return b
}

func (b *Builder) OverrideParam(name string, modifier ParamModifier) *Builder {
	b.paramModifiers[name] = modifier
	return b
}

// --- Public Marker Functions ---

func Route(routeRegistration interface{}) *Builder {
	return NewBuilder()
}

func Group(groupRegistration interface{}) *Builder {
	return NewBuilder()
}
