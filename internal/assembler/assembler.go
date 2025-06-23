package assembler

import (
	"fmt"
	"strings"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

func BuildSpec(apiModel *model.APIModel, cfg *config.Config) (*openapi3.T, error) {
	// ... (initial spec setup is unchanged) ...
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
	spec.Components.SecuritySchemes = cfg.GetSecuritySchemes()

	fmt.Println("Assembling specification from route graph...")
	addRoutesToSpec(spec, apiModel.RouteGraph, apiModel.GroupMetadata)

	fmt.Println("✅ Specification assembled successfully.")
	return spec, nil
}

func addRoutesToSpec(spec *openapi3.T, node *model.RouteNode, groupMetadata model.GroupMetadataMap) {
	// ... (group metadata application is unchanged) ...
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

		// ... (hierarchical metadata application is unchanged) ...
		var allTags []string
		var allSecurity []string
		for n := node; n != nil; n = n.Parent {
			allTags = append(allTags, n.Tags...)
			allSecurity = append(allSecurity, n.InferredSecurity...)
		}
		operationSpec.Tags = allTags
		if len(allSecurity) > 0 {
			req := openapi3.SecurityRequirement{}
			seenSchemes := make(map[string]bool)
			for _, schemeName := range allSecurity {
				if !seenSchemes[schemeName] {
					req[schemeName] = []string{}
					seenSchemes[schemeName] = true
				}
			}
			operationSpec.Security = &openapi3.SecurityRequirements{req}
		}

		// --- CORRECTED: Apply Handler-Specific Overrides from the Model ---
		if builder := op.BuilderMetadata; builder != nil {
			if s := builder.Summary; s != "" {
				operationSpec.Summary = s
			}
			if d := builder.Description; d != "" {
				operationSpec.Description = d
			}
			if t := builder.Tags; len(t) > 0 {
				operationSpec.Tags = t // Handler tags overwrite all group tags
			}
			if schemes := builder.Security; len(schemes) > 0 {
				req := openapi3.SecurityRequirement{}
				for _, schemeName := range schemes {
					req[schemeName] = []string{}
				}
				// This security requirement replaces any inferred ones.
				operationSpec.Security = &openapi3.SecurityRequirements{req}
			}
		}

		pathItem := spec.Paths.Find(op.FullPath)
		if pathItem == nil {
			pathItem = &openapi3.PathItem{}
			spec.Paths.Set(op.FullPath, pathItem)
		}

		pathItem.SetOperation(strings.ToUpper(op.HTTPMethod), operationSpec)
	}

	for _, child := range node.Children {
		addRoutesToSpec(spec, child, groupMetadata)
	}
}
