package analyzer

import (
	"go/ast"
	"go/types"
)

// KnownRouters defines the constructor functions for popular routers.
// The key is the package path, the value is the function name.
var KnownRouters = map[string]string{
	"github.com/gin-gonic/gin": "New",
	"github.com/go-chi/chi/v5": "NewRouter",
	"net/http":                 "NewServeMux",
}

// isRouterInitialization checks if a function call is a known router constructor from config.
func (a *Analyzer) isRouterInitialization(call *ast.CallExpr) bool {
	obj := a.getObjectForExpr(call.Fun)
	if obj == nil {
		return false
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}

	// Check if the function's return type is a configured router type.
	if sig := fn.Signature(); sig.Results().Len() == 1 {
		return a.isRouterType(sig.Results().At(0).Type())
	}
	return false
}

// findAssignStmt looks for the variable assignment for a router initialization.
func (a *Analyzer) findAssignStmt(file *ast.File, call *ast.CallExpr) types.Object {
	var assignedObj types.Object
	ast.Inspect(file, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			if len(assign.Rhs) == 1 && assign.Rhs[0] == call {
				if len(assign.Lhs) > 0 {
					assignedObj = a.getObjectForExpr(assign.Lhs[0])
				}
				return false // Stop searching once found
			}
		}
		return true
	})
	return assignedObj
}

// Helper to get type info for an expression
// func (a *Analyzer) getTypeInfo(expr ast.Expr) types.Info {
// 	for _, pkg := range a.pkgs {
// 		if info, ok := pkg.TypesInfo.Defs[expr.(*ast.Ident)]; ok {
// 			// this is simplified, needs to handle SelectorExpr etc.
// 		}
// 	}
// 	// This is a placeholder for a correct implementation that searches all info objects.
// 	// A real implementation would map files to packages to get the correct TypesInfo.
// 	// For now, we brute force it for demonstration.
// 	for _, pkg := range a.pkgs {
// 		for id, obj := range pkg.TypesInfo.Uses {
// 			if id.Pos() == expr.Pos() {
// 				// return a dummy info for now
// 			}
// 		}
// 	}
// 	return types.Info{} // dummy
// }

// getObjectForExpr is a robust helper to find the types.Object for any expression.
func (a *Analyzer) getObjectForExpr(expr ast.Expr) types.Object {
	info := a.fileTypeInfo[a.currentFile]
	if info == nil {
		return nil
	}

	switch e := expr.(type) {
	case *ast.Ident:
		if obj := info.Uses[e]; obj != nil {
			return obj
		}
		if obj := info.Defs[e]; obj != nil {
			return obj
		}
	case *ast.SelectorExpr:
		return info.Uses[e.Sel]
	// FEAT: Handle type conversions like http.HandlerFunc(myHandler)
	case *ast.CallExpr:
		// Check if this call is a type conversion.
		tv, ok := info.Types[e.Fun]
		if ok && tv.IsType() {
			// It is. Recurse on the argument inside the conversion to find the real handler.
			if len(e.Args) == 1 {
				return a.getObjectForExpr(e.Args[0])
			}
		}
	}
	return nil
}

// isRouterType checks if a given type matches any of the configured router definitions.
func (a *Analyzer) isRouterType(t types.Type) bool {
	for _, def := range a.routerDefs {
		if t.String() == def.Type {
			return true
		}
	}
	return false
}

// getRouteMethodType determines if a method name is an endpoint, group, or middleware wrapper.
func (a *Analyzer) getRouteMethodType(receiverType types.Type, methodName string) string {
	for _, def := range a.routerDefs {
		if receiverType.String() == def.Type {
			for _, m := range def.EndpointMethods {
				if m == methodName {
					return "endpoint"
				}
			}
			for _, m := range def.GroupMethods {
				if m == methodName {
					return "group"
				}
			}
			for _, m := range def.MiddlewareWrapperMethods {
				if m == methodName {
					return "middleware"
				}
			}
		}
	}
	return "unknown"
}
