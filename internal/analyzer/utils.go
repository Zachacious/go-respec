package analyzer

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

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

// getFuncPath constructs a fully qualified path for a function object.
// e.g., "net/http.ResponseWriter.Write" or "strconv.Atoi"
func getFuncPath(obj types.Object) string {
	if obj == nil {
		return ""
	}

	fn, ok := obj.(*types.Func)
	if !ok {
		// Not a function object, so it has no function path.
		return ""
	}

	// Check if it's a method by looking for a receiver.
	if sig := fn.Type().(*types.Signature); sig != nil && sig.Recv() != nil {
		// It's a method. The full name includes the receiver type.
		// sig.Recv().Type().String() correctly gives us the full type name, e.g., "*github.com/go-chi/chi/v5.Mux"
		return sig.Recv().Type().String() + "." + fn.Name()
	}

	// It's a regular function.
	if fn.Pkg() == nil {
		return ""
	}
	return fn.Pkg().Path() + "." + fn.Name()
}

// resolveIntValue attempts to resolve an expression to an integer value.
// It supports basic literals and constant expressions.
func (s *State) resolveIntValue(expr ast.Expr) (int, bool) {
	info := s.getInfoForNode(expr)
	if info == nil {
		return 0, false
	}

	// Case 1: It's a basic literal number, e.g., 201
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.INT {
		if val, err := strconv.Atoi(lit.Value); err == nil {
			return val, true
		}
	}

	// Case 2: It's an identifier for a constant, e.g., http.StatusOK
	if ident, ok := expr.(*ast.Ident); ok {
		if obj := info.Uses[ident]; obj != nil {
			if c, ok := obj.(*types.Const); ok {
				if val, exact := constant.Int64Val(c.Val()); exact {
					return int(val), true
				}
			}
		}
	}
	// Case 3: It's a selector for a constant, e.g. http.StatusOK
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if obj := info.Uses[sel.Sel]; obj != nil {
			if c, ok := obj.(*types.Const); ok {
				if val, exact := constant.Int64Val(c.Val()); exact {
					return int(val), true
				}
			}
		}
	}

	return 0, false
}

// resolveStringValue attempts to resolve an expression to a string value.
// It supports basic string literals, constant strings, and binary string concatenations.
func (s *State) resolveStringValue(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			val, err := strconv.Unquote(e.Value)
			if err == nil {
				return val, true
			}
		}
	case *ast.Ident:
		obj := s.getObjectForExpr(e)
		if constObj, isConst := obj.(*types.Const); isConst {
			val, err := strconv.Unquote(constObj.Val().String())
			if err == nil {
				return val, true
			}
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			left, lok := s.resolveStringValue(e.X)
			right, rok := s.resolveStringValue(e.Y)
			if lok && rok {
				return left + right, true
			}
		}
	}
	return "", false
}