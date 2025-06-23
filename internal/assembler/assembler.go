package assembler

import (
	"fmt"
	"strings"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
	"github.com/Zachacious/go-respec/respec"
	"github.com/getkin/kin-openapi/openapi3"
)

func BuildSpec(apiModel *model.APIModel, cfg *config.Config) (*openapi3.T, error) {
	spec := &openapi3.T{
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
	addRoutesToSpec(spec, apiModel.RouteGraph, apiModel.GroupMetadata)

	fmt.Println("✅ Specification assembled successfully.")
	return spec, nil
}

// addRoutesToSpec uses the new model.GroupMetadataMap type.
func addRoutesToSpec(spec *openapi3.T, node *model.RouteNode, groupMetadata model.GroupMetadataMap) {
	// First, apply metadata from `respec.Meta(r)` to the current node.
	if meta, ok := groupMetadata[node.GoVar]; ok {
		if tags := meta.GetTags(); len(tags) > 0 {
			node.Tags = append(node.Tags, tags...)
		}
		if security := meta.GetSecurity(); len(security) > 0 {
			node.InferredSecurity = append(node.InferredSecurity, security...)
		}
	}

	// Process operations at the current node
	for _, op := range node.Operations {
		operationSpec := op.Spec

		// --- Apply Hierarchical Metadata ---
		var allTags []string
		var allSecurity []string
		for n := node; n != nil; n = n.Parent {
			allTags = append(allTags, n.Tags...)
			allSecurity = append(allSecurity, n.InferredSecurity...)
		}
		operationSpec.Tags = allTags

		if len(allSecurity) > 0 {
			req := openapi3.SecurityRequirement{}
			// Use a map to handle duplicates gracefully
			seenSchemes := make(map[string]bool)
			for _, schemeName := range allSecurity {
				if !seenSchemes[schemeName] {
					req[schemeName] = []string{}
					seenSchemes[schemeName] = true
				}
			}
			operationSpec.Security = &openapi3.SecurityRequirements{req}
		}

		// --- Apply Handler-Specific Overrides (Highest Priority) ---
		if builder := respec.GetByHandler(op.GoHandler); builder != nil {
			if s := builder.GetSummary(); s != "" {
				operationSpec.Summary = s
			}
			if d := builder.GetDescription(); d != "" {
				operationSpec.Description = d
			}
			if t := builder.GetTags(); len(t) > 0 {
				operationSpec.Tags = t // Handler tags overwrite group tags
			}
			if schemes := builder.GetSecurity(); len(schemes) > 0 {
				req := openapi3.SecurityRequirement{}
				for _, schemeName := range schemes {
					req[schemeName] = []string{}
				}
				operationSpec.Security = &openapi3.SecurityRequirements{req}
			}
		}

		// Find or create the PathItem for this route.
		pathItem := spec.Paths.Find(op.FullPath)
		if pathItem == nil {
			pathItem = &openapi3.PathItem{}
			spec.Paths.Set(op.FullPath, pathItem)
		}

		pathItem.SetOperation(strings.ToUpper(op.HTTPMethod), operationSpec)
	}

	// Recurse into child nodes
	for _, child := range node.Children {
		addRoutesToSpec(spec, child, groupMetadata)
	}
}
