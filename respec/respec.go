// Package respec provides the annotation API for the respec OpenAPI generator.
//
// Everything in this package exists to be read by respec's static analyzer at
// build time. The builders do record what you pass them, but nothing in your
// program ever reads it back: at run time Handler is a pass-through and Meta
// does nothing. That is the point - your routing code keeps its normal shape
// and the documentation lives beside it instead of in comments.
//
//	r.Post("/users", respec.Handler(h.CreateUser).
//		Summary("Create a user").
//		Response(http.StatusCreated, respec.Schema(User{})).
//		Response(http.StatusBadRequest, respec.Desc("Invalid payload")).
//		Unwrap())
//
// This package deliberately has no dependencies outside the standard library,
// and imports nothing from go/ast, so adding it to a web service costs nothing
// at run time.
package respec

// --- Handler Builder ---

// HandlerBuilder carries documentation for a single operation. Create one with
// Handler and finish with Unwrap.
type HandlerBuilder[T any] struct {
	handler      T
	summary      string
	description  string
	tags         []string
	security     []string
	noSecurity   bool
	requestBody  any
	responses    map[int]responseSpec
	operationID  string
	deprecated   bool
	hidden       bool
	parameters   []paramSpec
	respHeaders  []headerSpec
	servers      []serverSpec
	externalDocs *docsSpec
	extensions   map[string]any
}

type responseSpec struct {
	description string
	content     any
}

type paramSpec struct {
	in, name, description, schema, example string
	required, deprecated                   bool
}

type headerSpec struct {
	code                      int
	name, description, schema string
}

type serverSpec struct{ url, description string }

type docsSpec struct{ url, description string }

// Handler wraps a handler so it can be documented. The handler's type is
// preserved, so Unwrap returns exactly what you passed in and any router
// accepts it.
func Handler[T any](handler T) *HandlerBuilder[T] {
	return &HandlerBuilder[T]{handler: handler}
}

// Unwrap returns the original handler. It must be the last call in the chain.
func (hb *HandlerBuilder[T]) Unwrap() T { return hb.handler }

// Summary sets the operation's one-line summary.
func (hb *HandlerBuilder[T]) Summary(s string) *HandlerBuilder[T] { hb.summary = s; return hb }

// Description sets the operation's long description.
func (hb *HandlerBuilder[T]) Description(d string) *HandlerBuilder[T] { hb.description = d; return hb }

// Tag adds tags, replacing any inherited from the enclosing group.
func (hb *HandlerBuilder[T]) Tag(tags ...string) *HandlerBuilder[T] {
	hb.tags = append(hb.tags, tags...)
	return hb
}

// Security names the security schemes this operation requires, replacing any
// inherited from the enclosing group.
func (hb *HandlerBuilder[T]) Security(schemeName ...string) *HandlerBuilder[T] {
	hb.security = append(hb.security, schemeName...)
	return hb
}

// Public clears any security requirement inherited from the enclosing group.
// Use it for endpoints such as login or health checks that sit inside an
// otherwise authenticated group.
func (hb *HandlerBuilder[T]) Public() *HandlerBuilder[T] { hb.noSecurity = true; return hb }

// RequestBody declares the request body schema from a value of the body's type.
func (hb *HandlerBuilder[T]) RequestBody(obj any) *HandlerBuilder[T] { hb.requestBody = obj; return hb }

// Response declares a response. Options compose, so a response can carry both a
// schema and a description:
//
//	.Response(400, respec.Schema(APIError{}), respec.Desc("Validation failed"))
func (hb *HandlerBuilder[T]) Response(code int, opts ...ResponseOption) *HandlerBuilder[T] {
	if hb.responses == nil {
		hb.responses = make(map[int]responseSpec)
	}
	spec := hb.responses[code]
	for _, opt := range opts {
		opt(&spec)
	}
	hb.responses[code] = spec
	return hb
}

// AddResponse declares a response from either a value (used as the schema) or a
// string (used as the description).
//
// Response is the fuller form and can express both at once; AddResponse remains
// because it reads well for the common case.
func (hb *HandlerBuilder[T]) AddResponse(code int, content any) *HandlerBuilder[T] {
	if s, ok := content.(string); ok {
		return hb.Response(code, Desc(s))
	}
	return hb.Response(code, Schema(content))
}

// OperationID sets the operation's unique identifier, used by client generators.
func (hb *HandlerBuilder[T]) OperationID(id string) *HandlerBuilder[T] {
	hb.operationID = id
	return hb
}

// Deprecated marks the operation as deprecated.
func (hb *HandlerBuilder[T]) Deprecated() *HandlerBuilder[T] { hb.deprecated = true; return hb }

// Deprecate marks the operation as deprecated when d is true.
//
// Deprecated: use Deprecated, which reads better and has no redundant argument.
func (hb *HandlerBuilder[T]) Deprecate(d bool) *HandlerBuilder[T] { hb.deprecated = d; return hb }

// Hide omits this operation from the generated spec. Use it for internal or
// debug endpoints that should not appear in published documentation.
func (hb *HandlerBuilder[T]) Hide() *HandlerBuilder[T] { hb.hidden = true; return hb }

// QueryParam declares a query parameter.
func (hb *HandlerBuilder[T]) QueryParam(name string, opts ...ParamOption) *HandlerBuilder[T] {
	return hb.param("query", name, opts)
}

// PathParam declares a path parameter. Path parameters are always required.
func (hb *HandlerBuilder[T]) PathParam(name string, opts ...ParamOption) *HandlerBuilder[T] {
	return hb.param("path", name, append([]ParamOption{Required()}, opts...))
}

// HeaderParam declares a header parameter.
func (hb *HandlerBuilder[T]) HeaderParam(name string, opts ...ParamOption) *HandlerBuilder[T] {
	return hb.param("header", name, opts)
}

// CookieParam declares a cookie parameter.
func (hb *HandlerBuilder[T]) CookieParam(name string, opts ...ParamOption) *HandlerBuilder[T] {
	return hb.param("cookie", name, opts)
}

func (hb *HandlerBuilder[T]) param(in, name string, opts []ParamOption) *HandlerBuilder[T] {
	spec := paramSpec{in: in, name: name}
	for _, opt := range opts {
		opt(&spec)
	}
	hb.parameters = append(hb.parameters, spec)
	return hb
}

// ResponseHeader declares a header returned with a given response code.
func (hb *HandlerBuilder[T]) ResponseHeader(code int, name string, opts ...HeaderOption) *HandlerBuilder[T] {
	spec := headerSpec{code: code, name: name, schema: "string"}
	for _, opt := range opts {
		opt(&spec)
	}
	hb.respHeaders = append(hb.respHeaders, spec)
	return hb
}

// AddServer adds an operation-level server URL.
func (hb *HandlerBuilder[T]) AddServer(url, desc string) *HandlerBuilder[T] {
	hb.servers = append(hb.servers, serverSpec{url: url, description: desc})
	return hb
}

// ExternalDocs links to documentation hosted elsewhere.
func (hb *HandlerBuilder[T]) ExternalDocs(url, desc string) *HandlerBuilder[T] {
	hb.externalDocs = &docsSpec{url: url, description: desc}
	return hb
}

// Extensions adds vendor extension fields. Keys should begin with "x-".
func (hb *HandlerBuilder[T]) Extensions(ext map[string]any) *HandlerBuilder[T] {
	if hb.extensions == nil {
		hb.extensions = make(map[string]any, len(ext))
	}
	for k, v := range ext {
		hb.extensions[k] = v
	}
	return hb
}

// --- Options ---

// ResponseOption configures a response declared with Response.
type ResponseOption func(*responseSpec)

// Schema sets a response's body schema from a value of the body's type.
func Schema(v any) ResponseOption {
	return func(r *responseSpec) { r.content = v }
}

// Desc sets a response's description.
func Desc(s string) ResponseOption {
	return func(r *responseSpec) { r.description = s }
}

// ParamOption configures a parameter.
type ParamOption func(*paramSpec)

// Required marks a parameter as required.
func Required() ParamOption {
	return func(p *paramSpec) { p.required = true }
}

// Doc sets a parameter's description.
func Doc(s string) ParamOption {
	return func(p *paramSpec) { p.description = s }
}

// Type sets a parameter's primitive type: string, integer, number, or boolean.
// The default is string.
func Type(t string) ParamOption {
	return func(p *paramSpec) { p.schema = t }
}

// Example sets a parameter's example value.
func Example(v string) ParamOption {
	return func(p *paramSpec) { p.example = v }
}

// DeprecatedParam marks a parameter as deprecated.
func DeprecatedParam() ParamOption {
	return func(p *paramSpec) { p.deprecated = true }
}

// HeaderOption configures a response header.
type HeaderOption func(*headerSpec)

// HeaderDoc sets a response header's description.
func HeaderDoc(s string) HeaderOption {
	return func(h *headerSpec) { h.description = s }
}

// HeaderType sets a response header's primitive type. The default is string.
func HeaderType(t string) HeaderOption {
	return func(h *headerSpec) { h.schema = t }
}

// --- Group Builder ---

// GroupBuilder carries documentation that applies to every route in a routing
// scope. Create one with Meta.
type GroupBuilder struct {
	tags       []string
	security   []string
	deprecated bool
	noSecurity bool
}

// Meta attaches documentation to a routing scope. The router argument
// identifies the scope to the analyzer; it is not used at run time.
//
//	r.Route("/users", func(r chi.Router) {
//		respec.Meta(r).Tag("Users").Security("BearerAuth")
//		...
//	})
func Meta(router any) *GroupBuilder { return &GroupBuilder{} }

// Tag adds tags to every route in this scope.
func (b *GroupBuilder) Tag(tags ...string) *GroupBuilder { b.tags = append(b.tags, tags...); return b }

// Security names the security schemes every route in this scope requires.
func (b *GroupBuilder) Security(schemeName ...string) *GroupBuilder {
	b.security = append(b.security, schemeName...)
	return b
}

// Public clears security inherited from enclosing scopes.
func (b *GroupBuilder) Public() *GroupBuilder { b.noSecurity = true; return b }

// Deprecated marks every route in this scope as deprecated.
func (b *GroupBuilder) Deprecated() *GroupBuilder { b.deprecated = true; return b }

// Deprecate marks every route in this scope as deprecated when d is true.
//
// Deprecated: use Deprecated, which reads better and has no redundant argument.
func (b *GroupBuilder) Deprecate(d bool) *GroupBuilder { b.deprecated = d; return b }
