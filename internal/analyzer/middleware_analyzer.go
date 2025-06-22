package analyzer

import (
	"go/ast"
	"go/types"
)

// analyzeMiddleware inspects a middleware function's body to infer properties like security.
func (s *State) analyzeMiddleware(middlewareObj types.Object) []string {
	var securitySchemes []string

	// Find the middleware function's declaration in our universe
	funcDecl, ok := s.Universe.Functions[middlewareObj]
	if !ok || funcDecl.Body == nil {
		return nil
	}

	// Inspect the function body for calls that match our security patterns
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		info := s.getInfoForNode(call.Fun)
		if info == nil {
			return true
		}

		var obj types.Object
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			obj = info.Uses[fun.Sel]
		case *ast.Ident:
			obj = info.Uses[fun]
		default:
			return true
		}
		if obj == nil {
			return true
		}

		var funcPath string
		if sig, ok := obj.Type().(*types.Signature); ok && sig.Recv() != nil {
			funcPath = sig.Recv().Type().String() + "." + obj.Name()
		} else if obj.Pkg() != nil {
			funcPath = obj.Pkg().Path() + "." + obj.Name()
		} else {
			return true
		}

		// Check against user-configured security patterns
		for _, p := range s.Config.SecurityPatterns {
			if funcPath == p.FunctionPath {
				securitySchemes = append(securitySchemes, p.SchemeName)
			}
		}
		return true
	})

	return securitySchemes
}
