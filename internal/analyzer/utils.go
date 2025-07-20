package analyzer

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/printer"
	"go/token"
	"go/types"
	"strconv"
	"strings"

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

// getObjectForExpr provides the definitive implementation for resolving an expression to its type object.
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
		if sel := info.Selections[e]; sel != nil {
			return sel.Obj()
		}
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
func getFuncPath(obj types.Object) string {
	if obj == nil {
		return ""
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		return ""
	}
	if sig := fn.Type().(*types.Signature); sig != nil && sig.Recv() != nil {
		// THIS IS THE FIX: Trim the leading '*' from pointer receiver types
		// to ensure it matches the path string defined in the .respec.yaml config.
		receiverType := sig.Recv().Type().String()
		return strings.TrimPrefix(receiverType, "*") + "." + fn.Name()
	}
	if fn.Pkg() == nil {
		return ""
	}
	return fn.Pkg().Path() + "." + fn.Name()
}

// resolveIntValue attempts to resolve an expression to an integer value.
// func (s *State) resolveIntValue(expr ast.Expr) (int, bool) {
// 	info := s.getInfoForNode(expr)
// 	if info == nil {
// 		return 0, false
// 	}
// 	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.INT {
// 		if val, err := strconv.Atoi(lit.Value); err == nil {
// 			return val, true
// 		}
// 	}
// 	if ident, ok := expr.(*ast.Ident); ok {
// 		if obj := info.Uses[ident]; obj != nil {
// 			if c, ok := obj.(*types.Const); ok {
// 				if val, exact := constant.Int64Val(c.Val()); exact {
// 					return int(val), true
// 				}
// 			}
// 		}
// 	}
// 	if sel, ok := expr.(*ast.SelectorExpr); ok {
// 		if obj := info.Uses[sel.Sel]; obj != nil {
// 			if c, ok := obj.(*types.Const); ok {
// 				if val, exact := constant.Int64Val(c.Val()); exact {
// 					return int(val), true
// 				}
// 			}
// 		}
// 	}
// 	return 0, false
// }

func (s *State) resolveIntValue(expr ast.Expr) (int, bool) {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.INT {
		if val, err := strconv.Atoi(lit.Value); err == nil {
			return val, true
		}
	}

	info := s.getInfoForNode(expr)
	if info == nil {
		return 0, false
	}

	if ident, ok := expr.(*ast.Ident); ok {
		if obj := info.Uses[ident]; obj != nil {
			if c, ok := obj.(*types.Const); ok && c.Val().Kind() == constant.Int {
				if val, exact := constant.Int64Val(c.Val()); exact {
					return int(val), true
				}
			}
		}
	}

	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if obj := info.Uses[sel.Sel]; obj != nil {
			if c, ok := obj.(*types.Const); ok && c.Val().Kind() == constant.Int {
				if val, exact := constant.Int64Val(c.Val()); exact {
					return int(val), true
				}
			}
		}
	}

	return 0, false
}

// resolveStringValue attempts to resolve an expression to a string value.
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

// SprintNode converts an AST node back to its string representation.
func (s *State) SprintNode(node ast.Node) string {
	if node == nil {
		return "<nil>"
	}
	var buf bytes.Buffer
	fset := s.Fset
	if fset == nil && len(s.pkgs) > 0 {
		fset = s.pkgs[0].Fset
	}
	if fset == nil {
		return "<error: no fset>"
	}
	err := printer.Fprint(&buf, fset, node)
	if err != nil {
		return "<error: " + err.Error() + ">"
	}
	return buf.String()
}
