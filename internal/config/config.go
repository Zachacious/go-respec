package config

import (
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

type SecurityPattern struct {
	FunctionPath string `yaml:"functionPath"`
	SchemeName   string `yaml:"schemeName"`
}

type ParameterPattern struct {
	FunctionPath string `yaml:"functionPath"`
	NameIndex    int    `yaml:"nameIndex"`
}

type RequestBodyPattern struct {
	FunctionPath string `yaml:"functionPath"`
	ArgIndex     int    `yaml:"argIndex"`
}

type ResponseBodyPattern struct {
	FunctionPath     string `yaml:"functionPath"`
	DataIndex        int    `yaml:"dataIndex"`
	StatusCodeIndex  *int   `yaml:"statusCodeIndex,omitempty"`
	DescriptionIndex *int   `yaml:"descriptionIndex,omitempty"`
}

type HandlerPatternsConfig struct {
	RequestBody     []RequestBodyPattern  `yaml:"requestBody"`
	ResponseBody    []ResponseBodyPattern `yaml:"responseBody"`
	QueryParameter  []ParameterPattern    `yaml:"queryParameter"`
	HeaderParameter []ParameterPattern    `yaml:"headerParameter"`
}

// --- END OF NEW CONFIG STRUCTS ---

// RouterDefinition is unchanged.
type RouterDefinition struct {
	Type                     string   `yaml:"type"`
	EndpointMethods          []string `yaml:"endpointMethods"`
	GroupMethods             []string `yaml:"groupMethods"`
	MiddlewareWrapperMethods []string `yaml:"middlewareWrapperMethods"`
}

// Config is updated with the new HandlerPatterns.
type Config struct {
	Info              *openapi3.Info         `yaml:"info"`
	SecuritySchemes   map[string]interface{} `yaml:"securitySchemes"`
	RouterDefinitions []RouterDefinition     `yaml:"routerDefinitions"`
	HandlerPatterns   *HandlerPatternsConfig `yaml:"handlerPatterns"`
	SecurityPatterns  []SecurityPattern      `yaml:"securityPatterns"`
}

// Load is updated with defaults for the new handler patterns.
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
				MiddlewareWrapperMethods: []string{"With"},
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
