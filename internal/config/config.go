package config

import (
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

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
}

// Config represents a configuration.
type Config struct {
	// Info is the information about the API.
	Info *openapi3.Info `yaml:"info"`
	// SecuritySchemes is a map of security schemes.
	SecuritySchemes map[string]interface{} `yaml:"securitySchemes"`
	// RouterDefinitions is a list of router definitions.
	RouterDefinitions []RouterDefinition `yaml:"routerDefinitions"`
	// HandlerPatterns is the handler patterns configuration.
	HandlerPatterns *HandlerPatternsConfig `yaml:"handlerPatterns"`
	// SecurityPatterns is a list of security patterns.
	SecurityPatterns []SecurityPattern `yaml:"securityPatterns"`
}

// Load loads a configuration from a file.
func Load(projectPath string) (*Config, error) {
	// Helper for making statusCodeIndex optional
	intPtr := func(i int) *int { return &i }

	cfg := &Config{
		Info: &openapi3.Info{Title: "API Documentation", Version: "1.0.0"},
		RouterDefinitions: []RouterDefinition{
			{
				Type:                     "github.com/go-chi/chi/v5.Mux",
				EndpointMethods:          []string{"Get", "Post", "Put", "Patch", "Delete", "Head", "Options", "Trace"},
				GroupMethods:             []string{"Route", "Group"},
				MiddlewareWrapperMethods: []string{"With", "Use"},
			},
			{
				Type:                     "github.com/gin-gonic/gin.Engine",
				EndpointMethods:          []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
				GroupMethods:             []string{"Group"},
				MiddlewareWrapperMethods: []string{},
			},
		},
		SecuritySchemes: make(map[string]any),
		SecurityPatterns: []SecurityPattern{
			// Add a default pattern for your project's token validation as an example
			{
				FunctionPath: "github.com/zachacious/justauth/internal/services/token.Service.Validate",
				SchemeName:   "BearerAuth",
			},
		},
		HandlerPatterns: &HandlerPatternsConfig{
			RequestBody: []RequestBodyPattern{
				// Add a default for the user's custom validation function
				{FunctionPath: "github.com/zachacious/justauth/internal/utils.ValidateRequest", ArgIndex: 0},
				// Standard library / common framework patterns
				{FunctionPath: "encoding/json.Decoder.Decode", ArgIndex: 0},
				{FunctionPath: "github.com/gin-gonic/gin.Context.ShouldBindJSON", ArgIndex: 0},
				{FunctionPath: "github.com/labstack/echo/v4.Context.Bind", ArgIndex: 0},
			},
			ResponseBody: []ResponseBodyPattern{
				// Standard library pattern
				{FunctionPath: "encoding/json.Encoder.Encode", DataIndex: 0},
				// Gin-like pattern
				{FunctionPath: "github.com/gin-gonic/gin.Context.JSON", StatusCodeIndex: intPtr(0), DataIndex: 1},
				// Common custom helper patterns (like in your project)
				{FunctionPath: "github.com/zachacious/justauth/internal/utils.RespondWithJSON", StatusCodeIndex: intPtr(1), DataIndex: 2},
				{FunctionPath: "github.com/zachacious/justauth/internal/utils.RespondWithError", StatusCodeIndex: intPtr(1), DescriptionIndex: intPtr(2), DataIndex: 3},
			},

			QueryParameter: []ParameterPattern{
				{FunctionPath: "net/http.URL.Query.Get", NameIndex: 0},
			},
			HeaderParameter: []ParameterPattern{
				{FunctionPath: "net/http.Header.Get", NameIndex: 0},
			},
		},
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
		schemes[key] = &openapi3.SecuritySchemeRef{Value: scheme}
	}
	return schemes
}
