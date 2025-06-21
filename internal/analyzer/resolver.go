package analyzer

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/Zachacious/go-respec/internal/config"
)

// resolveConfigTypes is Phase 1 of the analysis.
// It iterates through the router definitions in the config and resolves the
// string type paths into canonical, reliable *types.Named objects.
func (s *State) resolveConfigTypes(cfg *config.Config) error {
	fmt.Println("Phase 1: Resolving configured types...")
	for i := range cfg.RouterDefinitions {
		def := &cfg.RouterDefinitions[i] // Get a pointer to the definition
		namedType := s.findNamedType(def.Type)

		if namedType == nil {
			// This is a configuration error. The user specified a type that doesn't exist.
			fmt.Printf("  [Warning] Could not find type '%s' defined in .respec.yaml in the project's dependencies.\n", def.Type)
			continue
		}

		fmt.Printf("  [Info] Resolved type '%s'\n", def.Type)
		s.ResolvedRouterTypes[def.Type] = &ResolvedType{
			Object:     namedType,
			Definition: def,
		}
	}

	if len(s.ResolvedRouterTypes) == 0 {
		return fmt.Errorf("could not resolve any router types from .respec.yaml. Please check config and ensure dependencies are installed")
	}

	return nil
}

// findNamedType takes a full type path (e.g., "github.com/gin-gonic/gin.Engine")
// and resolves it to a canonical *types.Named object across all loaded packages.
func (s *State) findNamedType(typePath string) *types.Named {
	// Separate package path from type name
	lastDot := strings.LastIndex(typePath, ".")
	if lastDot == -1 {
		return nil // Invalid format (e.g., "int")
	}
	pkgPath := typePath[:lastDot]
	typeName := typePath[lastDot+1:]

	for _, pkg := range s.pkgs {
		if pkg.PkgPath == pkgPath {
			// Found the package. Now look for the type name in its scope.
			if obj := pkg.Types.Scope().Lookup(typeName); obj != nil {
				// We found the object. Is it a type name?
				if tn, ok := obj.(*types.TypeName); ok {
					// It is. The underlying type is the *types.Named we want.
					if named, ok := tn.Type().(*types.Named); ok {
						return named
					}
				}
			}
		}
	}
	return nil // Not found
}
