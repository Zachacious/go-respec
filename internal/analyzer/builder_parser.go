package analyzer

import (
	"go/ast"

	"github.com/Zachacious/go-respec/respec"
)

// FindAndParseRouteMetadata scans the AST for `respec.Handler(...).Unwrap()` call chains.
func (s *State) FindAndParseRouteMetadata() {
	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Unwrap" {
					return true
				}

				metadata, handlerExpr := s.parseHandlerChain(sel.X)
				if handlerExpr == nil {
					return true
				}

				if handlerObj := s.getObjectForExpr(handlerExpr); handlerObj != nil {
					s.OperationMetadata[handlerObj] = metadata
				}
				return false
			})
		}
	}
}

// parseHandlerChain walks a call chain backwards to parse metadata.
func (s *State) parseHandlerChain(expr ast.Expr) (*respec.HandlerMetadata, ast.Expr) {
	metadata := &respec.HandlerMetadata{
		ResponseExprs: make(map[int]ast.Expr),
	}
	currentExpr := expr

	for {
		call, isCall := currentExpr.(*ast.CallExpr)
		if !isCall {
			break
		}
		sel, isSel := call.Fun.(*ast.SelectorExpr)
		if !isSel {
			break
		}

		methodName := sel.Sel.Name

		switch methodName {
		case "Summary":
			if str, ok := s.resolveStringValue(call.Args[0]); ok {
				metadata.Summary = str
			}
		case "Description":
			if str, ok := s.resolveStringValue(call.Args[0]); ok {
				metadata.Description = str
			}
		case "Tag":
			for _, arg := range call.Args {
				if str, ok := s.resolveStringValue(arg); ok {
					metadata.Tags = append(metadata.Tags, str)
				}
			}
		case "Security":
			if str, ok := s.resolveStringValue(call.Args[0]); ok {
				metadata.Security = append(metadata.Security, str)
			}
		case "RequestBody":
			if len(call.Args) > 0 {
				metadata.RequestBodyExpr = call.Args[0]
			}
		case "AddResponse":
			if len(call.Args) == 2 {
				if code, ok := s.resolveIntValue(call.Args[0]); ok {
					metadata.ResponseExprs[code] = call.Args[1]
				}
			}
		case "OperationID":
			if str, ok := s.resolveStringValue(call.Args[0]); ok {
				metadata.OperationID = str
			}
		case "Deprecate":
			if val, ok := getBoolValue(call.Args[0]); ok {
				metadata.Deprecated = val
			}
		case "Handler":
			if len(call.Args) == 1 {
				return metadata, call.Args[0]
			}
			return nil, nil
		}
		currentExpr = sel.X
	}
	return nil, nil
}

// getBoolValue is a simple helper to resolve a boolean literal.
func getBoolValue(expr ast.Expr) (bool, bool) {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == "true", true
	}
	return false, false
}
