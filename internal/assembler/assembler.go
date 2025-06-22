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
	spec := &openapi3.T{
		// FIX: Update to a valid OpenAPI 3.1.0 version string.
		OpenAPI: "3.1.0",
		Info:    cfg.Info,
		Components: &openapi3.Components{
			Schemas:         make(openapi3.Schemas),
			SecuritySchemes: make(openapi3.SecuritySchemes),
		},
		Paths: &openapi3.Paths{},
	}

	spec.Components.Schemas = apiModel.Components.Schemas

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
		// Get the operation spec that was populated by the inference engine.
		operationSpec := op.Spec

		// Layer 2/3 - Inferred Security from Middleware
		// Collect security schemes by walking up the parent chain.
		var securityRequirements []map[string][]string
		for n := node; n != nil; n = n.Parent {
			for _, schemeName := range n.InferredSecurity {
				securityRequirements = append(securityRequirements, map[string][]string{
					schemeName: {},
				})
			}
		}
		if len(securityRequirements) > 0 {
			// Apply the inferred security. This can be overridden by the builder below.
			operationSpec.Security = &openapi3.SecurityRequirements{
				securityRequirements[0], // Simplified for now, just takes the first one found
			}
		}

		// Layer 1 - Explicit Overrides from Metadata Builder
		// If metadata from a respec.Route() wrapper exists, it has the highest
		// priority and will override any inferred or doc comment values.
		if op.BuilderMetadata != nil {
			meta := op.BuilderMetadata
			if s := meta.GetSummary(); s != "" {
				operationSpec.Summary = s
			}
			if d := meta.GetDescription(); d != "" {
				operationSpec.Description = d
			}
			if t := meta.GetTags(); len(t) > 0 {
				operationSpec.Tags = t
			}
			if schemes := meta.GetSecurity(); len(schemes) > 0 {
				// This creates a new security requirement based on the names
				// provided to the builder, e.g., .Security("BearerAuth")
				req := openapi3.SecurityRequirement{}
				for _, schemeName := range schemes {
					req[schemeName] = []string{}
				}
				// This OVERWRITES any security that was inferred from middleware.
				operationSpec.Security = &openapi3.SecurityRequirements{req}
			}
		}

		// Find or create the PathItem for this route.
		pathItem := spec.Paths.Find(op.FullPath)
		if pathItem == nil {
			pathItem = &openapi3.PathItem{}
			spec.Paths.Set(op.FullPath, pathItem)
		}

		// Use the SetOperation method to attach the final, combined operation spec.
		pathItem.SetOperation(strings.ToUpper(op.HTTPMethod), operationSpec)
	}

	// Recurse into child nodes
	for _, child := range node.Children {
		addRoutesToSpec(spec, child)
	}
}