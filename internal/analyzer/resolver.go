package analyzer

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/Zachacious/go-respec/internal/config"
	"golang.org/x/tools/go/packages"
)

func (s *State) resolveConfigTypes(cfg *config.Config) error {
	fmt.Println("Phase 1: Resolving configured types...")
	for i := range cfg.RouterDefinitions {
		def := &cfg.RouterDefinitions[i]
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

func (s *State) findNamedType(typePath string) *types.Named {
	visited := make(map[*packages.Package]bool)
	for _, pkg := range s.pkgs {
		if namedType := s.findTypeInPackageGraph(pkg, typePath, visited); namedType != nil {
			return namedType
		}
	}
	return nil
}

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
