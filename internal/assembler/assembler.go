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
		Paths: &openapi3.Paths{},
	}
	spec.Components.Schemas = apiModel.Components.Schemas

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
	}

	for _, op := range node.Operations {
		operationSpec := op.Spec

		// --- Corrected Metadata Assembly Logic ---

		// 1. Collect all hierarchical tags and security schemes by walking up the tree.
		var hierarchicalTags []string
		var hierarchicalSecurity []string
		for n := node; n != nil; n = n.Parent {
			hierarchicalTags = append(hierarchicalTags, n.Tags...)
			hierarchicalSecurity = append(hierarchicalSecurity, n.InferredSecurity...)
		}

		// 2. Start with the hierarchical data as the default.
		finalTags := hierarchicalTags
		finalSecurity := hierarchicalSecurity

		// 3. Apply explicit overrides from the .Handler() builder, which have the highest priority.
		if builder := op.HandlerMetadata; builder != nil {
			if s := builder.Summary; s != "" {
				operationSpec.Summary = s
			}
			if d := builder.Description; d != "" {
				operationSpec.Description = d
			}
			// Handler-level tags completely replace any inherited tags.
			if t := builder.Tags; len(t) > 0 {
				finalTags = t
			}
			// Handler-level security completely replaces any inherited security.
			if schemes := builder.Security; len(schemes) > 0 {
				finalSecurity = schemes
			}
		}

		// 4. Apply the final, calculated metadata to the operation, ensuring uniqueness.
		operationSpec.Tags = uniqueStrings(finalTags)
		if len(finalSecurity) > 0 {
			req := openapi3.SecurityRequirement{}
			for _, schemeName := range uniqueStrings(finalSecurity) {
				req[schemeName] = []string{}
			}
			operationSpec.Security = &openapi3.SecurityRequirements{req}
		}

		// --- End Corrected Logic ---

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

// uniqueStrings returns a slice with all duplicate strings removed, preserving order.
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
