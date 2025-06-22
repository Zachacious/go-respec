package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"strconv"

	"github.com/Zachacious/go-respec/respec"
)

// MetadataMap maps an inner route registration call expression to the builder metadata that wraps it.
type MetadataMap map[*ast.CallExpr]*respec.Builder

// FindAllMetadata is a new analysis phase that scans the entire project for
// `respec.Route()` and `respec.Group()` calls and parses their chained methods.
// 
// This phase populates the Metadata map with builder metadata for each route registration call.
func (s *State) FindAllMetadata() {
	fmt.Println("Phase 2.5: Parsing builder metadata...")
	s.Metadata = make(MetadataMap)

	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				// We are looking for the end of a chain, e.g., `.Summary("...")`
				callExpr, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				// Is this a call to a respec builder method?
				info := s.getInfoForNode(callExpr.Fun)
				if info == nil {
					return true
				}

				obj := s.getObjectForExpr(callExpr.Fun)
				if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != "github.com/Zachacious/go-respec/respec" {
					// Not a call to a function in the respec package.
					return true
				}

				// We found a chain. Parse it backwards from the end.
				builder, innerCall := parseBuilderChain(callExpr, info)
				if builder != nil && innerCall != nil {
					// We successfully parsed a chain. Store it in our map,
					// keyed by the actual route registration call inside.
					s.Metadata[innerCall] = builder
					// We've processed this entire chain, no need to inspect its children.
					return false
				}

				return true
			})
		}
	}
}

// parseBuilderChain walks a chain of calls like `respec.Route(...).Summary(...).Tag(...)` backwards.
// It returns the fully populated Builder object and a pointer to the inner call expression.
// 
// This function assumes that the input call expression is a valid chain of method calls.
func parseBuilderChain(endCall *ast.CallExpr, info *types.Info) (*respec.Builder, *ast.CallExpr) {
	builder := respec.NewBuilder()
	currentCall := endCall

	// Walk backwards up the chain of method calls
	for {
		selExpr, ok := currentCall.Fun.(*ast.SelectorExpr)
		if !ok {
			// We've reached the start of the chain, which isn't a method call.
			// This must be the initial `respec.Route()` or `respec.Group()` call.
			break
		}

		// What method is it?
		methodName := selExpr.Sel.Name

		// What is its argument?
		var argValue string
		if len(currentCall.Args) > 0 {
			// For simplicity, we assume a single string literal argument.
			if lit, ok := currentCall.Args[0].(*ast.BasicLit); ok {
				val, err := strconv.Unquote(lit.Value)
				if err == nil {
					argValue = val
				}
			}
		}

		// Populate the builder based on the method name.
		switch methodName {
		case "Summary":
			// Set the summary for the builder.
			builder.Summary(argValue)
		case "Description":
			// Set the description for the builder.
			builder.Description(argValue)
		case "Tag":
			// Add a tag to the builder.
			builder.Tag(argValue) // Assumes single tag, can be extended for variadic
		case "Security":
			// Set the security for the builder.
			builder.Security(argValue)
		}

		// Move to the previous call in the chain.
		// The receiver of the current call is the previous call expression.
		prevCall, ok := selExpr.X.(*ast.CallExpr)
		if !ok {
			// We've reached the end of the chainable methods.
			currentCall = nil
			break
		}
		currentCall = prevCall
	}

	// After the loop, currentCall should be the initial `respec.Route(...)` call.
	if currentCall == nil || len(currentCall.Args) == 0 {
		return nil, nil // Malformed chain
	}

	// The argument to `respec.Route` is the actual route registration call.
	innerCall, ok := currentCall.Args[0].(*ast.CallExpr)
	if !ok {
		return nil, nil
	}

	return builder, innerCall
}