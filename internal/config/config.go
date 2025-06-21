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
	Info              *openapi3.Info         `yaml:"info"`
	SecuritySchemes   map[string]interface{} `yaml:"securitySchemes"`
	RouterDefinitions []RouterDefinition     `yaml:"routerDefinitions"`
}

// Load finds and loads .respec.yaml, merging it with default values.
func Load(projectPath string) (*Config, error) {
	// Start with a robust set of default configurations.
	cfg := &Config{
		Info: &openapi3.Info{Title: "API Documentation", Version: "1.0.0"},
		// --- START OF FIX ---
		// Provide the actual default router definitions.
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
		// --- END OF FIX ---
		SecuritySchemes: make(map[string]interface{}),
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
