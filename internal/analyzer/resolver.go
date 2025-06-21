package analyzer

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/Zachacious/go-respec/internal/config"
	"golang.org/x/tools/go/packages"
)

// resolveConfigTypes remains the same, but will now succeed.
func (s *State) resolveConfigTypes(cfg *config.Config) error {
	fmt.Println("Phase 1: Resolving configured types...")
	for i := range cfg.RouterDefinitions {
		def := &cfg.RouterDefinitions[i]
		namedType := s.findNamedType(def.Type) // This will now search the full graph.

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

// findNamedType is now the entry point for the graph search.
func (s *State) findNamedType(typePath string) *types.Named {
	// visited map prevents infinite loops in case of cyclic imports.
	visited := make(map[*packages.Package]bool)

	for _, pkg := range s.pkgs {
		if namedType := s.findTypeInPackageGraph(pkg, typePath, visited); namedType != nil {
			return namedType
		}
	}
	return nil
}

// findTypeInPackageGraph performs a recursive, depth-first search of the package import graph.
func (s *State) findTypeInPackageGraph(pkg *packages.Package, typePath string, visited map[*packages.Package]bool) *types.Named {
	// 1. If we've seen this package before, stop to avoid cycles.
	if visited[pkg] {
		return nil
	}
	visited[pkg] = true

	// 2. Check the current package for the type.
	// We check the raw typePath and also a pointer-stripped version for matching.
	cleanTypePath := strings.TrimPrefix(typePath, "*")
	lastDot := strings.LastIndex(cleanTypePath, ".")
	if lastDot != -1 {
		pkgPath := cleanTypePath[:lastDot]
		typeName := cleanTypePath[lastDot+1:]

		if pkg.PkgPath == pkgPath {
			if obj := pkg.Types.Scope().Lookup(typeName); obj != nil {
				if tn, ok := obj.(*types.TypeName); ok {
					if named, ok := tn.Type().(*types.Named); ok {
						// Found it!
						return named
					}
				}
			}
		}
	}

	// 3. If not found, recurse into all imported packages.
	for _, imp := range pkg.Imports {
		if namedType := s.findTypeInPackageGraph(imp, typePath, visited); namedType != nil {
			// Propagate the successful result up the call stack.
			return namedType
		}
	}

	// 4. If not found anywhere in this branch of the graph, return nil.
	return nil
}
