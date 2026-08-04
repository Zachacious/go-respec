package analyzer

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/meta"
	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

// buildRouteFromCall turns one route registration call into an Operation.
//
// It never returns without either recording an operation or emitting a
// diagnostic. Dropping a route silently was the single most confusing failure
// mode respec had: the spec simply came out short, with nothing to explain it.
func (s *State) buildRouteFromCall(val *TrackedValue, call *ast.CallExpr, pattern config.EndpointPattern, methodName string) {
	path, httpMethods, ok := s.routePathAndMethods(call, pattern, methodName)
	if !ok {
		s.reportf(call.Pos(), "route registered by %s: the path is not a literal string, so this route was skipped", methodName)
		return
	}

	if pattern.HandlerArg >= len(call.Args) {
		s.reportf(call.Pos(), "route %q: expected a handler in argument %d but the call has %d arguments", path, pattern.HandlerArg, len(call.Args))
		return
	}

	handlerArg := call.Args[pattern.HandlerArg]
	resolved := s.resolveHandler(handlerArg)

	if resolved.Obj == nil && resolved.Body == nil {
		// The route is real even though the handler is opaque, so it still
		// belongs in the spec - just without inferred detail.
		s.reportf(call.Pos(),
			"route %q: could not resolve %s to a handler, so its request and response types were not inferred",
			path, s.SprintNode(handlerArg))
	}

	// resolved.Name may legitimately be empty for a handler respec could not
	// name. It is left empty so the operation simply has no summary, rather
	// than a placeholder that reads like a real one.
	name := resolved.Name

	// A path is written in the router's own syntax; OpenAPI only understands
	// the brace form, so normalise before anything reads the parameters.
	normalizedPath := val.RouterDef.NormalizePath(path)

	for _, httpMethod := range httpMethods {
		op := &model.Operation{
			HTTPMethod:      httpMethod,
			RoutePath:       normalizedPath,
			GoHandler:       resolved.Obj,
			HandlerName:     name,
			HandlerBody:     resolved.Body,
			Node:            val.Node,
			Spec:            newOperation(),
			HandlerMetadata: metadataOf(resolved.Site),
		}
		if resolved.Obj != nil && resolved.Obj.Pkg() != nil {
			op.HandlerPackage = resolved.Obj.Pkg().Path()
		}
		val.Node.Operations = append(val.Node.Operations, op)
	}
}

// newOperation builds an operation whose Responses map starts out empty.
//
// openapi3.NewOperation leaves Responses nil, and the first AddResponse call
// then lazily builds it with NewResponses(), which seeds a `default` entry with
// an empty description. That stray entry ended up on every operation respec
// ever emitted, so the map is created explicitly here instead.
func newOperation() *openapi3.Operation {
	op := openapi3.NewOperation()
	op.Responses = openapi3.NewResponsesWithCapacity(2)
	return op
}

func metadataOf(site *SpecSite) *meta.HandlerMetadata {
	if site == nil {
		return nil
	}
	return site.Metadata
}

// routePathAndMethods extracts the path and the HTTP methods a registration
// call covers, honouring the router's declared argument layout.
func (s *State) routePathAndMethods(call *ast.CallExpr, pattern config.EndpointPattern, methodName string) (string, []string, bool) {
	if pattern.PathArg >= len(call.Args) {
		return "", nil, false
	}
	rawPath, ok := s.resolveStringValue(call.Args[pattern.PathArg])
	if !ok {
		return "", nil, false
	}

	switch {
	case pattern.MethodInPath:
		// net/http ServeMux: "GET /users/{id}", or just "/users/{id}".
		verb, path := splitMethodPattern(rawPath)
		if verb == "" {
			// A verb-less pattern matches every method. Which of them to
			// document is a judgement the router definition owns, not the
			// analyzer, so it comes from config.
			methods := pattern.DefaultMethods
			if len(methods) == 0 {
				methods = []string{"GET"}
			}
			s.infof(call.Pos(),
				"pattern %q names no HTTP method so it matches all of them; documented as %s. Write %q to be explicit, .Hide() it, or set defaultMethods in .respec.yaml.",
				rawPath, strings.Join(methods, ", "), "GET "+rawPath)
			return path, methods, true
		}
		return path, []string{verb}, true

	case pattern.MethodArg != nil:
		if *pattern.MethodArg >= len(call.Args) {
			return "", nil, false
		}
		verb, ok := s.resolveStringValue(call.Args[*pattern.MethodArg])
		if !ok {
			return "", nil, false
		}
		return rawPath, []string{strings.ToUpper(verb)}, true

	default:
		return rawPath, []string{strings.ToUpper(methodName)}, true
	}
}

// splitMethodPattern separates the optional leading verb from a Go 1.22+
// ServeMux pattern. A host component, if present, is preserved in the path.
func splitMethodPattern(pattern string) (string, string) {
	parts := strings.Fields(pattern)
	if len(parts) == 2 && isHTTPMethod(parts[0]) {
		return strings.ToUpper(parts[0]), parts[1]
	}
	return "", pattern
}

func isHTTPMethod(s string) bool {
	switch strings.ToUpper(s) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE", "CONNECT":
		return true
	}
	return false
}

// addPathParameters declares a parameter for every placeholder in the final
// path. It runs after paths are assembled, because a mounted sub-router only
// learns its full path once its mount point is known.
func (s *State) addPathParameters(op *model.Operation) {
	for _, paramName := range config.PathParamNames(op.FullPath) {
		param := openapi3.NewPathParameter(paramName).WithSchema(openapi3.NewStringSchema())
		if op.HandlerBody != nil {
			s.inferPathParameterType(op.HandlerBody, param)
		}
		op.Spec.AddParameter(param)
	}
}

// inferPathParameterType scans a handler body to infer a more specific schema for a path parameter.
func (s *State) inferPathParameterType(body *ast.BlockStmt, param *openapi3.Parameter) {
	var paramVarObj types.Object

	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) == 0 || len(assign.Rhs) != 1 {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}

		selExpr, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		info := s.getInfoForNode(selExpr.Sel)
		if info == nil {
			return true
		}

		if funObj := info.Uses[selExpr.Sel]; funObj != nil && getFuncPath(funObj) == "github.com/go-chi/chi/v5.URLParam" {
			if len(call.Args) == 2 {
				if nameLit, ok := call.Args[1].(*ast.BasicLit); ok {
					if name, err := strconv.Unquote(nameLit.Value); err == nil && name == param.Name {
						if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
							paramVarObj = s.getInfoForNode(ident).Defs[ident]
							return false // Stop searching
						}
					}
				}
			}
		}
		return true
	})

	if paramVarObj == nil {
		return
	}

	ast.Inspect(body, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		info := s.getInfoForNode(ident)
		if info == nil || info.Uses[ident] != paramVarObj {
			return true
		}

		path, _ := s.findPathToNode(ident)
		if len(path) < 2 {
			return true
		}

		if call, ok := path[1].(*ast.CallExpr); ok {
			funInfo := s.getInfoForNode(call.Fun)
			if funInfo == nil {
				return true
			}

			var funObj types.Object
			if sel, isSel := call.Fun.(*ast.SelectorExpr); isSel {
				funObj = funInfo.Uses[sel.Sel]
			} else if funIdent, isIdent := call.Fun.(*ast.Ident); isIdent {
				funObj = funInfo.Uses[funIdent]
			}

			if funObj != nil {
				switch getFuncPath(funObj) {
				case "strconv.Atoi", "strconv.ParseInt":
					param.Schema.Value.Type = &openapi3.Types{"integer"}
					return false
				case "strconv.ParseFloat":
					param.Schema.Value.Type = &openapi3.Types{"number"}
					param.Schema.Value.Format = "double"
					return false
				case "github.com/google/uuid.Parse":
					param.Schema.Value.Format = "uuid"
					return false
				}
			}
		}
		return true
	})
}
