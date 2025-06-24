package analyzer

import (
	"go/ast"
	"strconv"

	"github.com/Zachacious/go-respec/respec"
)

// const respecHandlerFuncPath = "github.com/Zachacious/go-respec/respec.Handler"

// FindAndParseRouteMetadata scans the AST for `respec.Handler(...).Unwrap()` call chains,
// parses the metadata, and stores it in a map keyed by the handler's types.Object.
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

// parseHandlerChain walks a call chain backwards to parse metadata and find the root call.
func (s *State) parseHandlerChain(expr ast.Expr) (*respec.HandlerMetadata, ast.Expr) {
	metadata := &respec.HandlerMetadata{
		Tags:     []string{},
		Security: []string{},
	}
	currentExpr := expr

	// Walk backwards through a chain of calls like: A().B().C()
	for {
		call, ok := currentExpr.(*ast.CallExpr)
		if !ok {
			// This means we've reached the start of the chain.
			break
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			// This should not happen in a valid chain.
			break
		}

		methodName := sel.Sel.Name
		argValues := extractStringArgs(call)

		// This switch handles the metadata methods like .Tag(), .Security(), etc.
		// The case "Handler" is for the root of the chain.
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
		case "Handler":
			// This is the root call. The chain walk is complete.
			// The argument to this call is the actual handler function we need.
			if len(call.Args) == 1 {
				return metadata, call.Args[0]
			}
			return nil, nil // Invalid Handler() call.
		}

		// Move to the previous expression in the chain (the receiver of the call).
		currentExpr = sel.X
	}

	// If the loop completes without finding a "Handler" method, it's not a valid chain.
	return nil, nil
}

// extractStringArgs is a helper to get string literal values from call arguments.
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
