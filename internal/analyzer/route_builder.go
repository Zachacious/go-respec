package analyzer

import (
	"go/ast"
	"go/types"
	"regexp"
	"strconv"
	"strings"

	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

func (s *State) buildRouteFromCall(val *TrackedValue, call *ast.CallExpr, handlerDecl *ast.FuncDecl) {
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

	fullPath := s.assembleFullPath(val, path)
	if len(fullPath) > 1 && strings.HasSuffix(fullPath, "/") {
		fullPath = fullPath[:len(fullPath)-1]
	}

	handlerArg := call.Args[1]
	var handlerObj types.Object
	var originalHandlerDecl *ast.FuncDecl = handlerDecl
	finalHandlerExpr := handlerArg

	if callExpr, ok := handlerArg.(*ast.CallExpr); ok {
		if sel, ok := callExpr.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Unwrap" {
			if _, realHandler := s.parseHandlerChain(sel.X); realHandler != nil {
				finalHandlerExpr = realHandler
			}
		}
	}

	handlerObj = s.getObjectForExpr(finalHandlerExpr)

	if handlerObj == nil {
		return
	}

	if originalHandlerDecl == nil || s.getObjectForExpr(handlerArg) != handlerObj {
		originalHandlerDecl = s.Universe.Functions[handlerObj]
	}

	op := &model.Operation{
		HTTPMethod:  httpMethod,
		FullPath:    fullPath,
		GoHandler:   handlerObj,
		HandlerName: handlerObj.Name(),
		Spec:        openapi3.NewOperation(),
	}
	if handlerObj.Pkg() != nil {
		op.HandlerPackage = handlerObj.Pkg().Path()
	}

	if metadata, ok := s.OperationMetadata[handlerObj]; ok {
		op.HandlerMetadata = metadata
	}

	routeNode := val.Node
	routeNode.Operations = append(routeNode.Operations, op)

	re := regexp.MustCompile(`\{(\w+)\}`)
	matches := re.FindAllStringSubmatch(fullPath, -1)
	for _, match := range matches {
		if len(match) > 1 {
			paramName := match[1]
			param := openapi3.NewPathParameter(paramName).WithSchema(openapi3.NewStringSchema())
			if originalHandlerDecl != nil && originalHandlerDecl.Body != nil {
				s.inferPathParameterType(originalHandlerDecl.Body, param)
			}
			op.Spec.AddParameter(param)
		}
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

// assembleFullPath walks up the chain of tracked values to construct the
// complete path for an endpoint, prepending all parent prefixes.
func (s *State) assembleFullPath(val *TrackedValue, endpointPath string) string {
	var prefixes []string
	for current := val; current != nil; current = current.Parent {
		if current.PathPrefix != "" {
			prefixes = append(prefixes, current.PathPrefix)
		}
	}

	var finalPath strings.Builder
	for i := len(prefixes) - 1; i >= 0; i-- {
		finalPath.WriteString(strings.TrimSuffix(prefixes[i], "/"))
	}

	if !strings.HasPrefix(endpointPath, "/") {
		finalPath.WriteString("/")
	}
	finalPath.WriteString(endpointPath)

	if finalPath.Len() == 0 {
		return "/"
	}
	return finalPath.String()
}
