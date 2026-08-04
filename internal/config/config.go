package config

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

type ServerUrl struct {
	// URL is the server URL.
	URL string `yaml:"url"`
	// Description is the server description.
	Description string `yaml:"description,omitempty"`
}

// SecurityPattern represents a security pattern.
type SecurityPattern struct {
	// FunctionPath is the path to the function.
	FunctionPath string `yaml:"functionPath"`
	// SchemeName is the name of the scheme.
	SchemeName string `yaml:"schemeName"`
}

// ParameterPattern represents a parameter pattern.
type ParameterPattern struct {
	// FunctionPath is the path to the function.
	FunctionPath string `yaml:"functionPath"`
	// NameIndex is the index of the name.
	NameIndex int `yaml:"nameIndex"`
}

// RequestBodyPattern represents a request body pattern.
type RequestBodyPattern struct {
	// FunctionPath is the path to the function.
	FunctionPath string `yaml:"functionPath"`
	// ArgIndex is the index of the argument.
	ArgIndex int `yaml:"argIndex"`
}

// ResponseBodyPattern represents a response body pattern.
type ResponseBodyPattern struct {
	// FunctionPath is the path to the function.
	FunctionPath string `yaml:"functionPath"`
	// DataIndex is the index of the data.
	DataIndex int `yaml:"dataIndex"`
	// StatusCodeIndex is the index of the status code.
	StatusCodeIndex *int `yaml:"statusCodeIndex,omitempty"`
	// DescriptionIndex is the index of the description.
	DescriptionIndex *int `yaml:"descriptionIndex,omitempty"`
}

// HandlerPatternsConfig represents a handler patterns configuration.
type HandlerPatternsConfig struct {
	// RequestBody is a list of request body patterns.
	RequestBody []RequestBodyPattern `yaml:"requestBody"`
	// ResponseBody is a list of response body patterns.
	ResponseBody []ResponseBodyPattern `yaml:"responseBody"`
	// QueryParameter is a list of query parameter patterns.
	QueryParameter []ParameterPattern `yaml:"queryParameter"`
	// HeaderParameter is a list of header parameter patterns.
	HeaderParameter []ParameterPattern `yaml:"headerParameter"`
}

// EndpointPattern describes the argument layout of a route registration call.
//
// The common case needs no entry: any name listed in EndpointMethods is assumed
// to be `Verb(path, handler)` with the HTTP verb taken from the method name.
// Routers that pass the verb as an argument, or fold it into the path string,
// need an explicit pattern.
type EndpointPattern struct {
	// Methods are the Go method names this pattern applies to.
	Methods []string `yaml:"methods"`
	// MethodArg is the index of an argument holding the HTTP method, as in
	// echo's e.Add("GET", "/x", h). Nil means the verb comes from elsewhere.
	MethodArg *int `yaml:"methodArg,omitempty"`
	// PathArg is the index of the argument holding the route path.
	PathArg int `yaml:"pathArg"`
	// HandlerArg is the index of the argument holding the handler.
	HandlerArg int `yaml:"handlerArg"`
	// MethodInPath marks the path argument as carrying a leading HTTP method,
	// as in net/http's mux.HandleFunc("GET /users/{id}", h).
	MethodInPath bool `yaml:"methodInPath,omitempty"`
	// DefaultMethods are the HTTP methods to document when the registration
	// names none. Such a pattern matches every method, but emitting one
	// operation per verb turns a single static file tree into five endpoints,
	// so the default is a single GET. Set this to widen or narrow that.
	DefaultMethods []string `yaml:"defaultMethods,omitempty"`
}

// RouterDefinition represents a router definition.
type RouterDefinition struct {
	// Type is the type of the router.
	Type string `yaml:"type"`
	// EndpointMethods is a list of endpoint methods.
	EndpointMethods []string `yaml:"endpointMethods"`
	// GroupMethods is a list of group methods.
	GroupMethods []string `yaml:"groupMethods"`
	// MiddlewareWrapperMethods is a list of middleware wrapper methods.
	MiddlewareWrapperMethods []string `yaml:"middlewareWrapperMethods"`
	// MountMethods is a list of methods that attach an already-built sub-router
	// under a path prefix, such as chi's Mount. The first argument is the
	// prefix and the second is the sub-router.
	MountMethods []string `yaml:"mountMethods,omitempty"`
	// EndpointPatterns describes registration calls whose shape differs from the
	// default `Verb(path, handler)` covered by EndpointMethods. This is what
	// lets one config model chi's r.Get("/x", h), echo's e.Add("GET", "/x", h),
	// and net/http's mux.HandleFunc("GET /x", h) without framework-specific code.
	EndpointPatterns []EndpointPattern `yaml:"endpointPatterns,omitempty"`
	// PathParamStyle names the syntax this router uses for path parameters, so
	// paths can be normalised to the OpenAPI `{name}` form. Supported values
	// are "brace" (/users/{id}, the default) and "colon" (/users/:id).
	PathParamStyle string `yaml:"pathParamStyle,omitempty"`
}

// Path parameter styles understood by PathParamStyle.
const (
	PathParamBrace = "brace"
	PathParamColon = "colon"
)

// colonParam matches gin/echo/fiber style `:name` segments. The name stops at
// the next path separator so that `/users/:id/posts` yields just `id`.
var colonParam = regexp.MustCompile(`:(\w+)`)

// braceParam matches chi and net/http style `{name}` segments.
var braceParam = regexp.MustCompile(`\{(\w+)\}`)

// NormalizePath rewrites a framework-native path into OpenAPI path-template
// form. OpenAPI only understands `{name}`, so a gin route registered as
// `/users/:id` has to be emitted as `/users/{id}` or the path parameter can
// never be matched to its declaration.
func (d *RouterDefinition) NormalizePath(path string) string {
	if d.PathParamStyle == PathParamColon {
		return colonParam.ReplaceAllString(path, "{$1}")
	}
	return path
}

// PathParamNames returns the parameter names in an already-normalised path.
func PathParamNames(normalizedPath string) []string {
	matches := braceParam.FindAllStringSubmatch(normalizedPath, -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			names = append(names, m[1])
		}
	}
	return names
}

// Config represents a configuration.
type Config struct {
	// Info is the information about the API.
	Info *openapi3.Info `yaml:"info"`
	// SecuritySchemes is a map of security schemes.
	SecuritySchemes map[string]any `yaml:"securitySchemes"`
	// RouterDefinitions is a list of router definitions.
	RouterDefinitions []RouterDefinition `yaml:"routerDefinitions"`
	// HandlerPatterns is the handler patterns configuration.
	HandlerPatterns *HandlerPatternsConfig `yaml:"handlerPatterns"`
	// SecurityPatterns is a list of security patterns.
	SecurityPatterns []SecurityPattern `yaml:"securityPatterns"`
	// Servers is a list of server URLs.
	Servers []ServerUrl `yaml:"servers,omitempty"`
}

// defaultRouterDefinitions describes the routers respec understands out of the
// box. Nothing here is special-cased in the analyzer: each entry is the same
// config a user could write themselves for any other router.
func defaultRouterDefinitions() []RouterDefinition {
	verbs := []string{"Get", "Post", "Put", "Patch", "Delete", "Head", "Options", "Trace", "Connect"}
	upperVerbs := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE", "CONNECT"}
	methodArg0 := 0

	return []RouterDefinition{
		{
			// chi's Mux and the Router interface its sub-routers are typed as.
			Type:                     "github.com/go-chi/chi/v5.Mux",
			EndpointMethods:          verbs,
			GroupMethods:             []string{"Route", "Group"},
			MiddlewareWrapperMethods: []string{"With", "Use"},
			MountMethods:             []string{"Mount"},
			PathParamStyle:           PathParamBrace,
			EndpointPatterns: []EndpointPattern{
				{Methods: []string{"Method", "MethodFunc"}, MethodArg: &methodArg0, PathArg: 1, HandlerArg: 2},
				{Methods: []string{"Handle", "HandleFunc"}, PathArg: 0, HandlerArg: 1},
			},
		},
		{
			Type:                     "github.com/go-chi/chi/v5.Router",
			EndpointMethods:          verbs,
			GroupMethods:             []string{"Route", "Group"},
			MiddlewareWrapperMethods: []string{"With", "Use"},
			MountMethods:             []string{"Mount"},
			PathParamStyle:           PathParamBrace,
			EndpointPatterns: []EndpointPattern{
				{Methods: []string{"Method", "MethodFunc"}, MethodArg: &methodArg0, PathArg: 1, HandlerArg: 2},
				{Methods: []string{"Handle", "HandleFunc"}, PathArg: 0, HandlerArg: 1},
			},
		},
		{
			// gin registers on the Engine, but every group is a RouterGroup and
			// the Engine embeds one, so both types have to be tracked.
			Type:                     "github.com/gin-gonic/gin.Engine",
			EndpointMethods:          upperVerbs,
			GroupMethods:             []string{"Group"},
			MiddlewareWrapperMethods: []string{"Use"},
			PathParamStyle:           PathParamColon,
			EndpointPatterns: []EndpointPattern{
				{Methods: []string{"Handle"}, MethodArg: &methodArg0, PathArg: 1, HandlerArg: 2},
			},
		},
		{
			Type:                     "github.com/gin-gonic/gin.RouterGroup",
			EndpointMethods:          upperVerbs,
			GroupMethods:             []string{"Group"},
			MiddlewareWrapperMethods: []string{"Use"},
			PathParamStyle:           PathParamColon,
			EndpointPatterns: []EndpointPattern{
				{Methods: []string{"Handle"}, MethodArg: &methodArg0, PathArg: 1, HandlerArg: 2},
			},
		},
		{
			Type:                     "github.com/labstack/echo/v4.Echo",
			EndpointMethods:          upperVerbs,
			GroupMethods:             []string{"Group"},
			MiddlewareWrapperMethods: []string{"Use"},
			PathParamStyle:           PathParamColon,
			EndpointPatterns: []EndpointPattern{
				{Methods: []string{"Add"}, MethodArg: &methodArg0, PathArg: 1, HandlerArg: 2},
			},
		},
		{
			Type:                     "github.com/labstack/echo/v4.Group",
			EndpointMethods:          upperVerbs,
			GroupMethods:             []string{"Group"},
			MiddlewareWrapperMethods: []string{"Use"},
			PathParamStyle:           PathParamColon,
			EndpointPatterns: []EndpointPattern{
				{Methods: []string{"Add"}, MethodArg: &methodArg0, PathArg: 1, HandlerArg: 2},
			},
		},
		{
			// Go 1.22+ ServeMux folds the verb into the pattern:
			// mux.HandleFunc("GET /users/{id}", h)
			Type:           "net/http.ServeMux",
			PathParamStyle: PathParamBrace,
			EndpointPatterns: []EndpointPattern{
				{
					Methods:      []string{"Handle", "HandleFunc"},
					PathArg:      0,
					HandlerArg:   1,
					MethodInPath: true,
					// A verb-less ServeMux pattern matches every method. Most
					// are GET handlers or static file trees, so one GET is a
					// better default than five near-identical operations.
					DefaultMethods: []string{"GET"},
				},
			},
		},
	}
}

// Load loads a configuration from a file.
func Load(projectPath string) (*Config, error) {
	// Helper for making statusCodeIndex optional
	intPtr := func(i int) *int { return &i }

	cfg := &Config{
		Info:              &openapi3.Info{Title: "API Documentation", Version: "1.0.0"},
		RouterDefinitions: defaultRouterDefinitions(),
		SecuritySchemes:   make(map[string]any),
		SecurityPatterns:  []SecurityPattern{},
		HandlerPatterns: &HandlerPatternsConfig{
			RequestBody: []RequestBodyPattern{
				{FunctionPath: "encoding/json.Decoder.Decode", ArgIndex: 0},
				{FunctionPath: "github.com/gin-gonic/gin.Context.ShouldBindJSON", ArgIndex: 0},
				{FunctionPath: "github.com/gin-gonic/gin.Context.BindJSON", ArgIndex: 0},
				{FunctionPath: "github.com/labstack/echo/v4.Context.Bind", ArgIndex: 0},
			},
			ResponseBody: []ResponseBodyPattern{
				// encoding/json.Encoder.Encode is deliberately absent: it is
				// handled directly by the analyzer so that a preceding
				// WriteHeader call sets the status. Listing it here as well
				// would register a second, always-200 response for the same call.
				{FunctionPath: "github.com/gin-gonic/gin.Context.JSON", StatusCodeIndex: intPtr(0), DataIndex: 1},
				{FunctionPath: "github.com/gin-gonic/gin.Context.IndentedJSON", StatusCodeIndex: intPtr(0), DataIndex: 1},
				{FunctionPath: "github.com/labstack/echo/v4.Context.JSON", StatusCodeIndex: intPtr(0), DataIndex: 1},
			},

			QueryParameter: []ParameterPattern{
				// r.URL.Query() returns net/url.Values, so the Get call that
				// actually appears in handler bodies resolves to this path.
				{FunctionPath: "net/url.Values.Get", NameIndex: 0},
				{FunctionPath: "github.com/gin-gonic/gin.Context.Query", NameIndex: 0},
				{FunctionPath: "github.com/labstack/echo/v4.Context.QueryParam", NameIndex: 0},
			},
			HeaderParameter: []ParameterPattern{
				{FunctionPath: "net/http.Header.Get", NameIndex: 0},
				{FunctionPath: "github.com/gin-gonic/gin.Context.GetHeader", NameIndex: 0},
			},
		},
		Servers: []ServerUrl{},
	}

	configPath := filepath.Join(projectPath, ".respec.yaml")
	data, err := os.ReadFile(configPath)
	if err == nil {
		if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
			return nil, unmarshalErr
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	return cfg, nil
}

// parseOAuthFlows converts the loosely-typed YAML form of an oauth2 `flows`
// block into the openapi3 representation. Unknown flow names are ignored.
func parseOAuthFlows(raw any) *openapi3.OAuthFlows {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}

	flows := &openapi3.OAuthFlows{}
	assign := map[string]**openapi3.OAuthFlow{
		"implicit":          &flows.Implicit,
		"password":          &flows.Password,
		"clientCredentials": &flows.ClientCredentials,
		"authorizationCode": &flows.AuthorizationCode,
	}

	found := false
	for name, target := range assign {
		fm, ok := m[name].(map[string]any)
		if !ok {
			continue
		}
		flow := &openapi3.OAuthFlow{Scopes: make(map[string]string)}
		if v, ok := fm["authorizationUrl"].(string); ok {
			flow.AuthorizationURL = v
		}
		if v, ok := fm["tokenUrl"].(string); ok {
			flow.TokenURL = v
		}
		if v, ok := fm["refreshUrl"].(string); ok {
			flow.RefreshURL = v
		}
		if scopes, ok := fm["scopes"].(map[string]any); ok {
			for sk, sv := range scopes {
				if s, ok := sv.(string); ok {
					flow.Scopes[sk] = s
				}
			}
		}
		*target = flow
		found = true
	}

	if !found {
		return nil
	}
	return flows
}

// GetSecuritySchemes is a helper to get sanitized security schemes.
func (c *Config) GetSecuritySchemes() openapi3.SecuritySchemes {
	schemes := make(openapi3.SecuritySchemes)
	if c.SecuritySchemes == nil {
		return schemes
	}
	for key, val := range c.SecuritySchemes {
		schemeMap, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		scheme := &openapi3.SecurityScheme{}
		if t, ok := schemeMap["type"].(string); ok {
			scheme.Type = t
		}
		if d, ok := schemeMap["description"].(string); ok {
			scheme.Description = d
		}
		if s, ok := schemeMap["scheme"].(string); ok {
			scheme.Scheme = s
		}
		if bf, ok := schemeMap["bearerFormat"].(string); ok {
			scheme.BearerFormat = bf
		}
		// apiKey schemes are invalid without `in` and `name`, and openIdConnect
		// without its URL. Dropping them produced specs that respec's own
		// validate command rejected.
		if in, ok := schemeMap["in"].(string); ok {
			scheme.In = in
		}
		if n, ok := schemeMap["name"].(string); ok {
			scheme.Name = n
		}
		if u, ok := schemeMap["openIdConnectUrl"].(string); ok {
			scheme.OpenIdConnectUrl = u
		}
		if flows, ok := schemeMap["flows"]; ok {
			if parsed := parseOAuthFlows(flows); parsed != nil {
				scheme.Flows = parsed
			}
		}
		schemes[key] = &openapi3.SecuritySchemeRef{Value: scheme}
	}
	return schemes
}
