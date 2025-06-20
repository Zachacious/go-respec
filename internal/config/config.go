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

// Load looks for a .respec.yaml file and loads it, providing defaults.
func Load(projectPath string) (*Config, error) {
	configPath := filepath.Join(projectPath, ".respec.yaml")
	data, err := os.ReadFile(configPath)

	// If file doesn't exist, create a default config
	if os.IsNotExist(err) {
		return &Config{
			Info: &openapi3.Info{Title: "API Documentation", Version: "1.0.0"},
			RouterDefinitions: []RouterDefinition{
				{
					Type:                     "*github.com/go-chi/chi/v5.Mux",
					EndpointMethods:          []string{"Get", "Post", "Put", "Patch", "Delete", "Head", "Options", "Trace"},
					GroupMethods:             []string{"Route", "Group"},
					MiddlewareWrapperMethods: []string{"With"},
				},
				{
					Type:                     "*github.com/gin-gonic/gin.Engine",
					EndpointMethods:          []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
					GroupMethods:             []string{"Group"},
					MiddlewareWrapperMethods: []string{},
				},
			},
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
