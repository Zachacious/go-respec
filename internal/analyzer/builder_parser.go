package analyzer

import (
	"go/ast"
	"go/types"
	"strconv"

	"github.com/Zachacious/go-respec/respec"
)

const respecHandlerFuncPath = "github.com/Zachacious/go-respec/respec.Handler"

// FindAndParseRouteMetadata scans for `respec.Handler(...).Unwrap()` chains.
func (s *State) FindAndParseRouteMetadata() {
	s.OperationMetadata = make(map[types.Object]*respec.HandlerMetadata)

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

// parseHandlerChain walks a call chain backwards.
func (s *State) parseHandlerChain(expr ast.Expr) (*respec.HandlerMetadata, ast.Expr) {
	metadata := &respec.HandlerMetadata{
		Tags:     []string{},
		Security: []string{},
	}
	currentExpr := expr

	for {
		call, ok := currentExpr.(*ast.CallExpr)
		if !ok {
			break
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}
		methodName := sel.Sel.Name
		argValues := extractStringArgs(call)

		switch methodName {
		case "Summary":
			if len(argValues) > 0 {
				metadata.Summary = argValues[0]
			}
		case "Description":
			if len(argValues) > 0 {
				metadata.Description = argValues[0]
			}
		case "Tag":
			metadata.Tags = append(metadata.Tags, argValues...)
		case "Security":
			if len(argValues) > 0 {
				metadata.Security = append(metadata.Security, argValues[0])
			}
		}
		currentExpr = sel.X
	}

	handlerCall, ok := currentExpr.(*ast.CallExpr)
	if !ok {
		return nil, nil
	}
	obj := s.getObjectForExpr(handlerCall.Fun)
	if obj == nil {
		return nil, nil
	}
	if fn, ok := obj.(*types.Func); !ok || fn.FullName() != respecHandlerFuncPath {
		return nil, nil
	}

	if len(handlerCall.Args) == 1 {
		return metadata, handlerCall.Args[0]
	}

	return nil, nil
}

// extractStringArgs remains the same.
func extractStringArgs(call *ast.CallExpr) []string {
	var values []string
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok {
			if val, err := strconv.Unquote(lit.Value); err == nil {
				values = append(values, val)
			}
		}
	}
	return values
}
