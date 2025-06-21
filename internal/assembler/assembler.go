package assembler

import (
	"fmt"
	"strings"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

// BuildSpec constructs the final openapi3.T document from the analyzed API model.
func BuildSpec(apiModel *model.APIModel, cfg *config.Config) (*openapi3.T, error) {
	// FIX: Initialize the spec using a struct literal, as NewT() does not exist.
	// Also initialize nested pointer fields to prevent nil pointer issues.
	spec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    cfg.Info,
		Components: &openapi3.Components{
			Schemas:         make(openapi3.Schemas),
			SecuritySchemes: make(openapi3.SecuritySchemes),
		},
		Paths: &openapi3.Paths{},
	}

	// Populate the initialized fields from the analysis and config.
	spec.Components.Schemas = apiModel.Components.Schemas

	// Manually construct valid SecurityScheme objects from the generic map.
	sanitizedSchemes := make(openapi3.SecuritySchemes)
	if cfg.SecuritySchemes != nil {
		for key, val := range cfg.SecuritySchemes {
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

			sanitizedSchemes[key] = &openapi3.SecuritySchemeRef{Value: scheme}
		}
	}
	spec.Components.SecuritySchemes = sanitizedSchemes

	// Recursively traverse the route graph to populate the paths.
	fmt.Println("Assembling specification from route graph...")
	addRoutesToSpec(spec, apiModel.RouteGraph)

	fmt.Println("✅ Specification assembled successfully.")
	return spec, nil
}

// addRoutesToSpec is a recursive helper that traverses the RouteNode graph
// and adds all found operations to the specification's Paths object.
func addRoutesToSpec(spec *openapi3.T, node *model.RouteNode) {
	// Process operations at the current node
	for _, op := range node.Operations {
		// FIX: Use the library's intended methods for the Paths struct.
		pathItem := spec.Paths.Find(op.FullPath)
		if pathItem == nil {
			pathItem = &openapi3.PathItem{}
			// Use the Set method to add the new PathItem.
			spec.Paths.Set(op.FullPath, pathItem)
		}

		// Use the SetOperation method to attach the operation.
		pathItem.SetOperation(strings.ToUpper(op.HTTPMethod), op.Spec)
	}

	// Recurse into child nodes
	for _, child := range node.Children {
		addRoutesToSpec(spec, child)
	}
}
