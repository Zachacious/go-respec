package analyzer

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/ast/astutil"
)

// getInfoForNode is a helper to find the correct types.Info for any given node.
func (s *State) getInfoForNode(node ast.Node) *types.Info {
	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			if file.Pos() <= node.Pos() && node.End() <= file.End() {
				return s.fileTypeInfo[file]
			}
		}
	}
	return nil
}

// findPathToNode finds the chain of parent nodes from the file root to the target node.
func (s *State) findPathToNode(target ast.Node) ([]ast.Node, bool) {
	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			if file.Pos() <= target.Pos() && target.End() <= file.End() {
				path, exact := astutil.PathEnclosingInterval(file, target.Pos(), target.End())
				if exact {
					return path, true
				}
			}
		}
	}
	return nil, false
}

// isResolvedRouterType checks if a given type 't' matches one of the router types
// we resolved in Phase 1. It returns the corresponding ResolvedType if it's a match.
func (s *State) isResolvedRouterType(t types.Type) *ResolvedType {
	var named *types.Named
	if ptr, isPtr := t.(*types.Pointer); isPtr {
		if n, isNamed := ptr.Elem().(*types.Named); isNamed {
			named = n
		}
	} else if n, isNamed := t.(*types.Named); isNamed {
		named = n
	}

	if named == nil {
		return nil
	}

	for _, resolvedType := range s.ResolvedRouterTypes {
		if named.Obj() == resolvedType.Object.Obj() {
			return resolvedType
		}
	}
	return nil
}

// getObjectForExpr is a robust helper to find the types.Object for any expression
// that resolves to a named entity (a variable, function, etc.).
func (s *State) getObjectForExpr(expr ast.Expr) types.Object {
	info := s.getInfoForNode(expr)
	if info == nil {
		return nil
	}

	switch e := expr.(type) {
	case *ast.Ident:
		if obj := info.Uses[e]; obj != nil {
			return obj
		}
		return info.Defs[e]
	case *ast.SelectorExpr:
		return info.Uses[e.Sel]
	case *ast.CallExpr:
		tv, ok := info.Types[e.Fun]
		if ok && tv.IsType() && len(e.Args) == 1 {
			return s.getObjectForExpr(e.Args[0])
		}
	}
	return nil
}
