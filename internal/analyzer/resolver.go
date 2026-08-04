package analyzer

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/Zachacious/go-respec/internal/config"
	"golang.org/x/tools/go/packages"
)

// resolveConfigTypes resolves the types specified in the configuration.
// It iterates over the router definitions in the config, finds the corresponding
// named types in the project's dependencies, and stores them in the ResolvedRouterTypes map.
func (s *State) resolveConfigTypes(cfg *config.Config) error {
	var resolved []string

	for i := range cfg.RouterDefinitions {
		def := &cfg.RouterDefinitions[i]
		namedType := s.findNamedType(def.Type)

		// A router type that the project does not import is the normal case:
		// respec ships definitions for several frameworks and a project uses
		// one. Only the complete absence of a match is worth reporting.
		if namedType == nil {
			continue
		}

		resolved = append(resolved, def.Type)
		s.ResolvedRouterTypes[def.Type] = &ResolvedType{
			Object:     namedType,
			Definition: def,
		}
	}

	if len(s.ResolvedRouterTypes) == 0 {
		return fmt.Errorf(
			"no supported router type found in this project.\n"+
				"respec looked for: %s\n"+
				"If you use a different router, describe it under routerDefinitions in .respec.yaml",
			strings.Join(configuredTypeNames(cfg), ", "))
	}

	s.ResolvedRouterNames = resolved
	return nil
}

func configuredTypeNames(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.RouterDefinitions))
	for _, def := range cfg.RouterDefinitions {
		names = append(names, def.Type)
	}
	return names
}

// findNamedType searches for a named type in the project's dependencies.
// It iterates over the packages and uses findTypeInPackageGraph to search for the type.
func (s *State) findNamedType(typePath string) *types.Named {
	visited := make(map[*packages.Package]bool)
	for _, pkg := range s.pkgs {
		if namedType := s.findTypeInPackageGraph(pkg, typePath, visited); namedType != nil {
			return namedType
		}
	}
	return nil
}

// findTypeInPackageGraph searches for a named type in a package graph.
// It recursively searches the package and its imports for the type.
func (s *State) findTypeInPackageGraph(pkg *packages.Package, typePath string, visited map[*packages.Package]bool) *types.Named {
	if pkg == nil || visited[pkg] {
		return nil
	}
	visited[pkg] = true

	cleanTypePath := strings.TrimPrefix(typePath, "*")
	lastDot := strings.LastIndex(cleanTypePath, ".")
	if lastDot == -1 {
		return nil
	}
	pkgPath := cleanTypePath[:lastDot]
	typeName := cleanTypePath[lastDot+1:]

	if pkg.PkgPath == pkgPath {
		if obj := pkg.Types.Scope().Lookup(typeName); obj != nil {
			if tn, ok := obj.(*types.TypeName); ok {
				if named, ok := tn.Type().(*types.Named); ok {
					return named
				}
			}
		}
	}

	for _, imp := range pkg.Imports {
		if namedType := s.findTypeInPackageGraph(imp, typePath, visited); namedType != nil {
			return namedType
		}
	}

	return nil
}
