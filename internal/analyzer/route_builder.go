package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strconv"
	"strings"

	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

// buildRouteFromCall is the entry point for Phase 4. It's called by the data
// flow engine when a route registration method call (a "sink") is found.
func (s *State) buildRouteFromCall(val *TrackedValue, call *ast.CallExpr) {
	selExpr, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	httpMethod := strings.ToUpper(selExpr.Sel.Name)
	if len(call.Args) < 2 {
		return
	}

	pathArg := call.Args[0]
	path, ok := s.resolveStringValue(pathArg)
	if !ok {
		return
	}

	// Assemble the full path and normalize it
	fullPath := s.assembleFullPath(val, path)
	// FIX: Remove trailing slash if it's not the root path
	if len(fullPath) > 1 && strings.HasSuffix(fullPath, "/") {
		fullPath = fullPath[:len(fullPath)-1]
	}

	handlerArg := call.Args[1]
	handlerObj := s.getObjectForExpr(handlerArg)
	if handlerObj == nil {
		return
	}

	op := &model.Operation{
		HTTPMethod: httpMethod, FullPath: fullPath, GoHandler: handlerObj, HandlerName: handlerObj.Name(),
	}
	if handlerObj.Pkg() != nil {
		op.HandlerPackage = handlerObj.Pkg().Path()
	}

	// This is now an operation on a specific node.
	routeNode := val.Node
	routeNode.Operations = append(routeNode.Operations, op)

	// --- NEW: Auto-detect path parameters ---
	op.Spec = openapi3.NewOperation() // Init the spec object
	re := regexp.MustCompile(`\{(\w+)\}`)
	matches := re.FindAllStringSubmatch(fullPath, -1)
	for _, match := range matches {
		if len(match) > 1 {
			paramName := match[1]
			param := openapi3.NewPathParameter(paramName).WithSchema(openapi3.NewStringSchema())
			op.Spec.AddParameter(param)
		}
	}
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
