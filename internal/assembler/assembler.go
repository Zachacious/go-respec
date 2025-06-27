package assembler

import (
	"fmt"
	"strings"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

func BuildSpec(apiModel *model.APIModel, cfg *config.Config) (*openapi3.T, error) {
	spec := &openapi3.T{
		OpenAPI: "3.1.0",
		Info:    cfg.Info,
		Components: &openapi3.Components{
			Schemas:         make(openapi3.Schemas),
			SecuritySchemes: cfg.GetSecuritySchemes(),
		},
		Paths:   &openapi3.Paths{},
		Servers: make([]*openapi3.Server, 0, len(cfg.Servers)),
	}
	spec.Components.Schemas = apiModel.Components.Schemas
	// Add servers from the configuration
	for _, server := range cfg.Servers {
		spec.Servers = append(spec.Servers, &openapi3.Server{
			URL:         server.URL,
			Description: server.Description,
		})
	}

	fmt.Println("Assembling specification from route graph...")
	addRoutesToSpec(spec, apiModel.RouteGraph, apiModel.GroupMetadata)

	fmt.Println("✅ Specification assembled successfully.")
	return spec, nil
}

func addRoutesToSpec(spec *openapi3.T, node *model.RouteNode, groupMetadata model.GroupMetadataMap) {
	// Apply hierarchical metadata from respec.Meta calls to the current node.
	if meta, ok := groupMetadata[node.GoVar]; ok {
		if tags := meta.GetTags(); len(tags) > 0 {
			node.Tags = append(node.Tags, tags...)
		}
		if security := meta.GetSecurity(); len(security) > 0 {
			node.InferredSecurity = append(node.InferredSecurity, security...)
		}
		if meta.GetDeprecated() {
			node.Deprecated = true
		}
	}

	for _, op := range node.Operations {
		operationSpec := op.Spec

		// --- Layer 3 & 2: Inferred/Hierarchical Tags & Security ---
		var hierarchicalTags []string
		var hierarchicalSecurity []string
		var isDeprecated bool
		for n := node; n != nil; n = n.Parent {
			hierarchicalTags = append(hierarchicalTags, n.Tags...)
			hierarchicalSecurity = append(hierarchicalSecurity, n.InferredSecurity...)
			if n.Deprecated {
				isDeprecated = true
			}
		}
		// Apply unique hierarchical tags by default.
		operationSpec.Tags = uniqueStrings(hierarchicalTags)
		if isDeprecated && !operationSpec.Deprecated {
			operationSpec.Deprecated = true
		}

		// --- Layer 1: Explicit Overrides from .Handler() ---
		hasExplicitSecurityOverride := false
		if builder := op.HandlerMetadata; builder != nil {
			if s := builder.Summary; s != "" {
				operationSpec.Summary = s
			}
			if d := builder.Description; d != "" {
				operationSpec.Description = d
			}
			if t := builder.Tags; len(t) > 0 {
				// Handler-level tags completely overwrite hierarchical tags.
				operationSpec.Tags = uniqueStrings(t)
			}
			if schemes := builder.Security; len(schemes) > 0 {
				// Handler-level security completely overwrites any other security.
				hasExplicitSecurityOverride = true
				req := openapi3.SecurityRequirement{}
				for _, schemeName := range schemes {
					req[schemeName] = []string{}
				}
				operationSpec.Security = &openapi3.SecurityRequirements{req}
			}
			// Deprecation on Handler overrides group deprecation
			if builder.Deprecated {
				operationSpec.Deprecated = true
			}
		}

		// Apply unique hierarchical security ONLY if there wasn't an explicit override.
		if !hasExplicitSecurityOverride && len(hierarchicalSecurity) > 0 {
			req := openapi3.SecurityRequirement{}
			for _, schemeName := range uniqueStrings(hierarchicalSecurity) {
				req[schemeName] = []string{}
			}
			operationSpec.Security = &openapi3.SecurityRequirements{req}
		}

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

// uniqueStrings returns a slice with all duplicate strings removed.
func uniqueStrings(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(input))
	// Use a new slice to preserve the original order of the first occurrence.
	j := 0
	output := make([]string, len(input))
	for _, s := range input {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			output[j] = s
			j++
		}
	}
	return output[:j]
}
