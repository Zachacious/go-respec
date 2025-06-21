// Package config handles loading and parsing the .respec.yaml configuration file.
package config

import (
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

// RouterDefinition allows users to teach `respec` about their routing library.
type RouterDefinition struct {
	Type                     string   `yaml:"type"`
	EndpointMethods          []string `yaml:"endpointMethods"`
	GroupMethods             []string `yaml:"groupMethods"`
	MiddlewareWrapperMethods []string `yaml:"middlewareWrapperMethods"`
}

// Config represents the structure of the .respec.yaml file.
type Config struct {
	Info              *openapi3.Info                         `yaml:"info"`
	SecuritySchemes   map[string]*openapi3.SecuritySchemeRef `yaml:"securitySchemes"`
	RouterDefinitions []RouterDefinition                     `yaml:"routerDefinitions"`
}

// Load finds and loads .respec.yaml, merging it with default values.
func Load(projectPath string) (*Config, error) {
	// Start with a robust set of default configurations.
	cfg := &Config{
		Info: &openapi3.Info{Title: "API Documentation", Version: "1.0.0"},
		RouterDefinitions: []RouterDefinition{
			{
				// Note: Pointer '*' is removed from the type string.
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
		SecuritySchemes: make(map[string]*openapi3.SecuritySchemeRef),
	}

	configPath := filepath.Join(projectPath, ".respec.yaml")
	data, err := os.ReadFile(configPath)

	// If a config file exists, unmarshal it ON TOP of the defaults.
	// Fields present in the YAML will overwrite the defaults.
	// Fields not present in the YAML (like routerDefinitions) will keep their default values.
	if err == nil {
		if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
			return nil, unmarshalErr
		}
	} else if !os.IsNotExist(err) {
		// If there was an error other than the file not existing, return it.
		return nil, err
	}

	return cfg, nil
}
