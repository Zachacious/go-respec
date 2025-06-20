package assembler

import (
	"sort"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

func BuildSpec(apiModel *model.APIModel, cfg *config.Config) (*openapi3.T, error) {
	spec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    cfg.Info,
		Paths:   openapi3.NewPaths(),
		// FIX: Assign a pointer to the struct literal.
		Components: &openapi3.Components{
			Schemas:         apiModel.Components.Schemas,
			SecuritySchemes: cfg.SecuritySchemes,
		},
	}

	tempPaths := make(map[string]*openapi3.PathItem)
	buildPathsFromGraph(tempPaths, apiModel.RouteGraph)

	sortedKeys := make([]string, 0, len(tempPaths))
	for k := range tempPaths {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, k := range sortedKeys {
		spec.Paths.Set(k, tempPaths[k])
	}

	return spec, nil
}

func buildPathsFromGraph(paths map[string]*openapi3.PathItem, node *model.RouteNode) {
	if node == nil {
		return
	}
	for _, op := range node.Operations {
		pathItem := paths[op.FullPath]
		if pathItem == nil {
			pathItem = &openapi3.PathItem{}
			paths[op.FullPath] = pathItem
		}

		// Apply Builder Metadata Overrides
		if op.BuilderMetadata != nil {
			b := op.BuilderMetadata
			if summary := b.GetSummary(); summary != "" {
				op.Spec.Summary = summary
			}
			if description := b.GetDescription(); description != "" {
				op.Spec.Description = description
			}
			if tags := b.GetTags(); len(tags) > 0 {
				op.Spec.Tags = tags
			}

			// FEAT: Implement Security logic
			if schemes := b.GetSecurity(); len(schemes) > 0 {
				op.Spec.Security = &openapi3.SecurityRequirements{}
				for _, schemeName := range schemes {
					requirement := openapi3.NewSecurityRequirement()
					requirement[schemeName] = []string{}
					*op.Spec.Security = append(*op.Spec.Security, requirement)
				}
			}

			// FEAT: Implement OverrideParam logic
			if modifiers := b.GetParamModifiers(); len(modifiers) > 0 {
				for paramName, modifierFunc := range modifiers {
					for _, paramRef := range op.Spec.Parameters {
						if paramRef.Value != nil && paramRef.Value.Name == paramName {
							modifierFunc(paramRef.Value)
						}
					}
				}
			}
		}

		switch op.HTTPMethod {
		case "GET":
			pathItem.Get = op.Spec
		case "POST":
			pathItem.Post = op.Spec
		case "PUT":
			pathItem.Put = op.Spec
		case "DELETE":
			pathItem.Delete = op.Spec
		case "PATCH":
			pathItem.Patch = op.Spec
		case "HEAD":
			pathItem.Head = op.Spec
		}
	}
	for _, child := range node.Children {
		buildPathsFromGraph(paths, child)
	}
}
