package analyzer

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/Zachacious/go-respec/internal/config"
)

// resolveConfigTypes is Phase 1 of the analysis.
func (s *State) resolveConfigTypes(cfg *config.Config) error {
	fmt.Println("Phase 1: Resolving configured types...")
	for i := range cfg.RouterDefinitions {
		def := &cfg.RouterDefinitions[i]
		// The findNamedType function now only needs to find the base type.
		namedType := s.findNamedType(def.Type)

		if namedType == nil {
			fmt.Printf("  [Warning] Could not find type '%s' defined in config in the project's dependencies.\n", def.Type)
			continue
		}

		fmt.Printf("  [Info] Resolved type '%s'\n", def.Type)
		s.ResolvedRouterTypes[def.Type] = &ResolvedType{
			Object:     namedType,
			Definition: def,
		}
	}

	if len(s.ResolvedRouterTypes) == 0 {
		return fmt.Errorf("could not resolve any router types from config. Please check config and ensure dependencies are installed")
	}

	return nil
}

// findNamedType takes a full type path (e.g., "github.com/gin-gonic/gin.Engine")
// and resolves it to a canonical *types.Named object across all loaded packages.
// It no longer needs to handle pointers; it just finds the named type.
func (s *State) findNamedType(typePath string) *types.Named {
	lastDot := strings.LastIndex(typePath, ".")
	if lastDot == -1 {
		return nil
	}
	pkgPath := typePath[:lastDot]
	typeName := typePath[lastDot+1:]

	for _, pkg := range s.pkgs {
		if pkg.PkgPath == pkgPath {
			if obj := pkg.Types.Scope().Lookup(typeName); obj != nil {
				if tn, ok := obj.(*types.TypeName); ok {
					if named, ok := tn.Type().(*types.Named); ok {
						return named
					}
				}
			}
		}
	}
	return nil
}
