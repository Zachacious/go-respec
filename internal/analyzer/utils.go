package analyzer

import (
	"go/ast"
	"go/types"
)

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
	// This case handles `http.HandlerFunc(myHandler)`. It finds the object for `myHandler`.
	case *ast.CallExpr:
		tv, ok := info.Types[e.Fun]
		if ok && tv.IsType() && len(e.Args) == 1 {
			// It's a type conversion. Recurse on the argument inside.
			return s.getObjectForExpr(e.Args[0])
		}
	}
	return nil
}
