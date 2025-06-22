package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

// discoverUniverse is Phase 2 of the analysis.
// It scans all files in all packages to build a map of every top-level
// function and constant declaration.
func (s *State) discoverUniverse() {
	fmt.Println("Phase 2: Discovering project universe (functions and constants)...")

	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			// Set the current file context for the type info lookup
			info := s.fileTypeInfo[file]
			if info == nil {
				continue
			}

			// Inspect all top-level declarations in the file
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					// This is a function declaration.
					s.registerFunction(info, d)
				case *ast.GenDecl:
					// This could be an import, const, var, or type declaration.
					// We only care about constants for now.
					if d.Tok == token.CONST {
						s.registerConstants(info, d)
					}
				}
			}
		}
	}
	fmt.Printf("  [Info] Discovered %d functions and %d constants.\n", len(s.Universe.Functions), len(s.Universe.Constants))
}

// registerFunction records a function declaration in the universe map.
func (s *State) registerFunction(info *types.Info, funcDecl *ast.FuncDecl) {
	// The function name identifier gives us access to the types.Object
	if funcDecl.Name == nil {
		return // Should not happen for top-level functions
	}
	obj := info.Defs[funcDecl.Name]
	if obj == nil {
		return // Could be a function without a body, etc.
	}

	// We key the map by the canonical types.Object for the function.
	s.Universe.Functions[obj] = funcDecl
}

// registerConstants records all constant declarations from a GenDecl block.
func (s *State) registerConstants(info *types.Info, genDecl *ast.GenDecl) {
	for _, spec := range genDecl.Specs {
		if vs, ok := spec.(*ast.ValueSpec); ok {
			// A single `const` line can declare multiple constants (e.g., const a, b = 1, 2)
			for _, name := range vs.Names {
				obj := info.Defs[name]
				if obj != nil {
					// We key by the constant's object and store the entire value spec,
					// which includes the value itself.
					s.Universe.Constants[obj] = vs
				}
			}
		}
	}
}