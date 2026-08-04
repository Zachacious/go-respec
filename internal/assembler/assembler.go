package assembler

import (
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

	addRoutesToSpec(spec, apiModel.RouteGraph, apiModel.GroupMetadata)
	return spec, nil
}

func addRoutesToSpec(spec *openapi3.T, node *model.RouteNode, groupMetadata model.GroupMetadataMap) {
	// Apply hierarchical metadata from respec.Meta calls to the current node.
	if group, ok := groupMetadata[node.GoVar]; ok {
		if tags := group.Tags; len(tags) > 0 {
			node.Tags = append(node.Tags, tags...)
		}
		if security := group.Security; len(security) > 0 {
			node.InferredSecurity = append(node.InferredSecurity, security...)
		}
		if group.Deprecated {
			node.Deprecated = true
		}
		if group.NoSecurity {
			node.NoSecurity = true
		}
	}

	for _, op := range node.Operations {
		// .Hide() keeps internal endpoints out of published documentation.
		if op.HandlerMetadata != nil && op.HandlerMetadata.Hidden {
			continue
		}
		operationSpec := op.Spec

		// --- Layer 3 & 2: Inferred/Hierarchical Tags & Security ---
		var hierarchicalTags []string
		var hierarchicalSecurity []string
		var isDeprecated bool
		groupIsPublic := false
		for n := node; n != nil; n = n.Parent {
			hierarchicalTags = append(hierarchicalTags, n.Tags...)
			if n.Deprecated {
				isDeprecated = true
			}
			// A scope marked Public shields itself and everything under it from
			// security declared further up. Only security stops here: tags and
			// deprecation still come down from the enclosing groups, because
			// Public says nothing about how the endpoint is categorised.
			if n.NoSecurity {
				groupIsPublic = true
			}
			if !groupIsPublic {
				hierarchicalSecurity = append(hierarchicalSecurity, n.InferredSecurity...)
			}
		}

		// Tags follow the same three layers as everything else: an explicit
		// .Tag() beats an @tags doc line, which beats tags inherited from the
		// enclosing groups.
		tags := uniqueStrings(hierarchicalTags)
		if len(op.DocTags) > 0 {
			tags = uniqueStrings(op.DocTags)
		}
		operationSpec.Tags = tags

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
			// .Public() opts an endpoint out of the group's security, which is
			// how login and health routes live inside an authenticated group.
			if builder.NoSecurity {
				hasExplicitSecurityOverride = true
				operationSpec.Security = &openapi3.SecurityRequirements{}
			}
		}

		// A group marked Public states the absence of security explicitly, so
		// that a reader can tell it apart from a group nobody has secured yet.
		if !hasExplicitSecurityOverride && groupIsPublic && len(hierarchicalSecurity) == 0 {
			hasExplicitSecurityOverride = true
			operationSpec.Security = &openapi3.SecurityRequirements{}
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
