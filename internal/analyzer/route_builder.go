package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/Zachacious/go-respec/internal/model"
)

// buildRouteFromCall is the entry point for Phase 4. It's called by the data
// flow engine when a route registration method call (a "sink") is found.
func (s *State) buildRouteFromCall(item WorklistItem, call *ast.CallExpr) {
	selExpr, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	// 1. Determine HTTP Method
	httpMethod := strings.ToUpper(selExpr.Sel.Name)

	// 2. Resolve URL Path from the first argument
	if len(call.Args) < 2 {
		return // Not a valid route call (needs at least path and handler)
	}
	pathArg := call.Args[0]
	path, ok := s.resolveStringValue(pathArg)
	if !ok {
		fmt.Printf("  [Warning] Could not statically resolve path for call at %v\n", pathArg.Pos())
		return
	}

	// 2.5. Assemble the full path by walking the parent chain
	fullPath := s.assembleFullPath(item.Value, path)

	// 3. Resolve Handler Function from the second argument
	handlerArg := call.Args[1]
	handlerObj := s.getObjectForExpr(handlerArg)
	if handlerObj == nil {
		fmt.Printf("  [Warning] Could not resolve handler function for call at %v\n", handlerArg.Pos())
		return
	}

	// 4. Create and add the Operation to the model
	op := &model.Operation{
		HTTPMethod:  httpMethod,
		FullPath:    fullPath, // Use the fully assembled path
		GoHandler:   handlerObj,
		HandlerName: handlerObj.Name(),
	}
	if handlerObj.Pkg() != nil {
		op.HandlerPackage = handlerObj.Pkg().Path()
	}

	// Attach the operation to the correct node in the graph.
	routeNode := item.Value.Node
	routeNode.Operations = append(routeNode.Operations, op)

	fmt.Printf("      [Route] Created operation: %s %s -> %s\n", op.HTTPMethod, op.FullPath, op.HandlerName)
}

// resolveStringValue attempts to find the static string value of an expression.
// It can handle basic string literals and constants from the universe.
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
			// --- Start of fix ---
			val, err := strconv.Unquote(constObj.Val().String())
			if err == nil {
				return val, true
			}
			// --- End of fix ---
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

// assembleFullPath walks up the chain of tracked values to construct the
// complete path for an endpoint, prepending all parent prefixes.
func (s *State) assembleFullPath(val *TrackedValue, endpointPath string) string {
	var prefixes []string
	// Walk up the chain, collecting prefixes in reverse order.
	for current := val; current != nil; current = current.Parent {
		if current.PathPrefix != "" {
			prefixes = append(prefixes, current.PathPrefix)
		}
	}

	// Build the final path by joining the prefixes (in correct order) and the endpoint path.
	var finalPath strings.Builder
	for i := len(prefixes) - 1; i >= 0; i-- {
		finalPath.WriteString(strings.TrimSuffix(prefixes[i], "/"))
	}

	// Ensure there's a slash before the endpoint path if needed.
	if !strings.HasPrefix(endpointPath, "/") {
		finalPath.WriteString("/")
	}
	finalPath.WriteString(endpointPath)

	// Handle the case of an empty path resulting in just "/"
	if finalPath.Len() == 0 {
		return "/"
	}
	return finalPath.String()
}
