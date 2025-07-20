package respec

import (
	"go/ast"
)

// --- Internal Override Data Structures ---

type ParameterOverride struct {
	In          string
	Name        string
	Description string
	Required    bool
	Deprecated  bool
}

type ResponseHeaderOverride struct {
	Code        int
	Name        string
	Description string
}

type ResponseOverride struct {
	Code        int
	Description string
	ContentExpr ast.Expr
}

type ServerOverride struct {
	URL         string
	Description string
}

type ExternalDocsOverride struct {
	URL         string
	Description string
}

// --- Handler Builder ---

type HandlerBuilder[T any] struct {
	handler      T
	summary      string
	description  string
	tags         []string
	security     []string
	requestBody  any
	responses    map[int]any
	operationID  string
	deprecated   bool
	parameters   []ParameterOverride
	respHeaders  []ResponseHeaderOverride
	servers      []ServerOverride
	externalDocs *ExternalDocsOverride
	extentions   map[string]any
}

func Handler[T any](handler T) *HandlerBuilder[T] {
	return &HandlerBuilder[T]{
		handler:   handler,
		responses: make(map[int]any),
	}
}

func (hb *HandlerBuilder[T]) Unwrap() T                               { return hb.handler }
func (hb *HandlerBuilder[T]) Summary(s string) *HandlerBuilder[T]     { hb.summary = s; return hb }
func (hb *HandlerBuilder[T]) Description(d string) *HandlerBuilder[T] { hb.description = d; return hb }
func (hb *HandlerBuilder[T]) Tag(tags ...string) *HandlerBuilder[T] {
	hb.tags = append(hb.tags, tags...)
	return hb
}
func (hb *HandlerBuilder[T]) Security(schemeName ...string) *HandlerBuilder[T] {
	hb.security = append(hb.security, schemeName...)
	return hb
}
func (hb *HandlerBuilder[T]) RequestBody(obj any) *HandlerBuilder[T] { hb.requestBody = obj; return hb }
func (hb *HandlerBuilder[T]) AddResponse(code int, content any) *HandlerBuilder[T] {
	hb.responses[code] = content
	return hb
}
func (hb *HandlerBuilder[T]) OperationID(id string) *HandlerBuilder[T] {
	hb.operationID = id
	return hb
}
func (hb *HandlerBuilder[T]) Deprecate(d bool) *HandlerBuilder[T] { hb.deprecated = d; return hb }
func (hb *HandlerBuilder[T]) AddParameter(in, name, desc string, req, dep bool) *HandlerBuilder[T] {
	hb.parameters = append(hb.parameters, ParameterOverride{In: in, Name: name, Description: desc, Required: req, Deprecated: dep})
	return hb
}
func (hb *HandlerBuilder[T]) ResponseHeader(code int, name, desc string) *HandlerBuilder[T] {
	hb.respHeaders = append(hb.respHeaders, ResponseHeaderOverride{Code: code, Name: name, Description: desc})
	return hb
}
func (hb *HandlerBuilder[T]) AddServer(url, desc string) *HandlerBuilder[T] {
	hb.servers = append(hb.servers, ServerOverride{URL: url, Description: desc})
	return hb
}
func (hb *HandlerBuilder[T]) ExternalDocs(url, desc string) *HandlerBuilder[T] {
	hb.externalDocs = &ExternalDocsOverride{URL: url, Description: desc}
	return hb
}

func (hb *HandlerBuilder[T]) Extensions(ext map[string]any) *HandlerBuilder[T] {
	if hb.extentions == nil {
		hb.extentions = make(map[string]any)
	}
	for k, v := range ext {
		hb.extentions[k] = v
	}
	return hb
}

// --- Group Builder ---

type GroupBuilder struct {
	tags       []string
	security   []string
	deprecated bool
}

func NewGroupBuilder() *GroupBuilder                     { return &GroupBuilder{} }
func (b *GroupBuilder) GetTags() []string                { return b.tags }
func (b *GroupBuilder) GetSecurity() []string            { return b.security }
func (b *GroupBuilder) GetDeprecated() bool              { return b.deprecated }
func (b *GroupBuilder) Tag(tags ...string) *GroupBuilder { b.tags = append(b.tags, tags...); return b }
func (b *GroupBuilder) Security(schemeName ...string) *GroupBuilder {
	b.security = append(b.security, schemeName...)
	return b
}
func (b *GroupBuilder) Deprecate(d bool) *GroupBuilder { b.deprecated = d; return b }
func Meta(router interface{}) *GroupBuilder            { return NewGroupBuilder() }

// --- Internal Metadata Structure for Analyzer ---

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
}
