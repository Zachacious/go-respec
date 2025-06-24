package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"net/http"
	"strconv"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

type responseInfo struct {
	Type        types.Type
	Description string
}

func (s *State) analyzeHandlers() {
	fmt.Println("Phase 5: Analyzing handlers and generating schemas...")
	s.traverseAndAnalyze(s.RouteGraph)
}

// traverseAndAnalyze traverses the route graph and analyzes each handler.
func (s *State) traverseAndAnalyze(node *model.RouteNode) {
	for _, op := range node.Operations {
		s.analyzeHandlerBody(op)
	}
	for _, child := range node.Children {
		s.traverseAndAnalyze(child)
	}
}

// analyzeHandlerBody analyzes the body of a handler.
func (s *State) analyzeHandlerBody(op *model.Operation) {
	if op.Spec == nil {
		op.Spec = openapi3.NewOperation()
	}

	funcDecl, ok := s.Universe.Functions[op.GoHandler]
	if !ok {
		op.Spec.Summary = op.HandlerName
		op.Spec.AddResponse(200, openapi3.NewResponse().WithDescription("Successful response"))
		return
	}

	// --- Layer 2: Doc Comment Inference ---
	if funcDecl.Doc != nil {
		if parsedComment := parseDocComment(funcDecl.Doc); parsedComment != nil {
			op.Spec.Summary = parsedComment.Summary
			op.Spec.Description = parsedComment.Description
			// Note: Tags from doc comments will be merged in the assembler
		}
	}
	if op.Spec.Summary == "" {
		op.Spec.Summary = op.HandlerName
	}

	// --- Layer 3: Type Inference ---
	reqType := s.findRequestSchema(funcDecl.Body, s.Config.HandlerPatterns.RequestBody)
	if reqType != nil {
		schemaRef := s.SchemaGen.GenerateSchema(reqType)
		reqBody := openapi3.NewRequestBody().WithContent(openapi3.NewContentWithJSONSchemaRef(schemaRef))
		op.Spec.RequestBody = &openapi3.RequestBodyRef{Value: reqBody}
	}

	pathParamNames := make(map[string]bool)
	for _, p := range op.Spec.Parameters {
		if p.Value != nil && p.Value.In == "path" {
			pathParamNames[p.Value.Name] = true
		}
	}
	queryParams := s.findParametersByPattern(funcDecl.Body, s.Config.HandlerPatterns.QueryParameter, "query", pathParamNames)
	headerParams := s.findParametersByPattern(funcDecl.Body, s.Config.HandlerPatterns.HeaderParameter, "header", nil)
	for _, p := range queryParams {
		op.Spec.AddParameter(p.Value)
	}
	for _, p := range headerParams {
		op.Spec.AddParameter(p.Value)
	}

	responses := s.findResponseSchemas(funcDecl.Body, s.Config.HandlerPatterns.ResponseBody)
	for statusCode, info := range responses {
		var schemaRef *openapi3.SchemaRef
		if info.Type != nil {
			schemaRef = s.SchemaGen.GenerateSchema(info.Type)
		}
		desc := info.Description
		if desc == "" {
			desc = http.StatusText(statusCode)
		}
		if desc == "" {
			desc = "Response"
		}
		response := openapi3.NewResponse().WithDescription(desc)
		if statusCode != 204 && schemaRef != nil {
			response.WithContent(openapi3.NewContentWithJSONSchemaRef(schemaRef))
		}
		op.Spec.AddResponse(statusCode, response)
	}

	if op.Spec.Responses == nil || len(op.Spec.Responses.Map()) == 0 {
		op.Spec.AddResponse(200, openapi3.NewResponse().WithDescription("Successful response"))
	}

	// --- Layer 1: Apply Explicit Overrides ---
	if metadata, ok := s.OperationMetadata[op.GoHandler]; ok {
		if metadata.RequestBodyExpr != nil {
			if tv, ok := s.getInfoForNode(metadata.RequestBodyExpr).Types[metadata.RequestBodyExpr]; ok {
				schemaRef := s.SchemaGen.GenerateSchema(tv.Type)
				reqBody := openapi3.NewRequestBody().WithContent(openapi3.NewContentWithJSONSchemaRef(schemaRef))
				op.Spec.RequestBody = &openapi3.RequestBodyRef{Value: reqBody}
			}
		}
		for code, expr := range metadata.ResponseExprs {
			if tv, ok := s.getInfoForNode(expr).Types[expr]; ok {
				schemaRef := s.SchemaGen.GenerateSchema(tv.Type)
				response := openapi3.NewResponse().WithDescription(http.StatusText(code)).WithContent(openapi3.NewContentWithJSONSchemaRef(schemaRef))
				op.Spec.AddResponse(code, response)
			}
		}
		if metadata.OperationID != "" {
			op.Spec.OperationID = metadata.OperationID
		}
		if metadata.Deprecated {
			op.Spec.Deprecated = true
		}
	}
}

// findParametersByPattern finds parameters based on a given pattern.
func (s *State) findParametersByPattern(body *ast.BlockStmt, patterns []config.ParameterPattern, in string, exclusions map[string]bool) []*openapi3.ParameterRef {
	var params []*openapi3.ParameterRef
	foundParams := make(map[string]bool)

	ast.Inspect(body, func(n ast.Node) bool {
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

		// Before asserting the type, we must check it. An object could be a
		// function (*types.Func) or a variable that holds a function (*types.Var).
		var funcSignature *types.Signature
		var isSignature bool

		if fn, ok := obj.(*types.Func); ok {
			funcSignature, isSignature = fn.Type().(*types.Signature)
		} else if v, ok := obj.(*types.Var); ok {
			// If it's a variable, its underlying type might be a function signature
			funcSignature, isSignature = v.Type().Underlying().(*types.Signature)
		}

		// If we didn't find a function signature, we can't proceed with this object.
		if !isSignature {
			return true
		}

		var funcPath string
		if recv := funcSignature.Recv(); recv != nil {
			funcPath = recv.Type().String() + "." + obj.Name()
		} else if obj.Pkg() != nil {
			funcPath = obj.Pkg().Path() + "." + obj.Name()
		} else {
			return true
		}

		for _, p := range patterns {
			if funcPath == p.FunctionPath {
				if len(call.Args) > p.NameIndex {
					arg := call.Args[p.NameIndex]
					if key, ok := arg.(*ast.BasicLit); ok && key.Kind == token.STRING {
						paramName, err := strconv.Unquote(key.Value)
						if err != nil {
							return true
						}

						if !foundParams[paramName] && (exclusions == nil || !exclusions[paramName]) {
							var param *openapi3.Parameter
							switch in {
							case "query":
								param = openapi3.NewQueryParameter(paramName)
							case "header":
								param = openapi3.NewHeaderParameter(paramName)
							default:
								return true
							}
							param.WithSchema(openapi3.NewStringSchema())
							params = append(params, &openapi3.ParameterRef{Value: param})
							foundParams[paramName] = true
						}
					}
				}
			}
		}
		return true
	})
	return params
}

// findRequestSchema finds the request schema for a handler.
func (s *State) findRequestSchema(body *ast.BlockStmt, patterns []config.RequestBodyPattern) types.Type {
	var reqType types.Type
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		info := s.getInfoForNode(call.Fun)
		if info == nil {
			return true
		}

		obj := s.getObjectForExpr(call.Fun)
		if obj == nil {
			return true
		}

		funcPath := getFuncPath(obj)
		for _, p := range patterns {
			if funcPath == p.FunctionPath {
				if len(call.Args) > p.ArgIndex {
					arg := call.Args[p.ArgIndex]
					if info := s.getInfoForNode(arg); info != nil {
						if tv, ok := info.Types[arg]; ok {
							if ptr, isPtr := tv.Type.(*types.Pointer); isPtr {
								reqType = ptr.Elem()
								return false // Stop searching
							}
						}
					}
				}
			}
		}
		return true
	})
	return reqType
}

// findResponseSchemas finds response schemas for a handler.
func (s *State) findResponseSchemas(body *ast.BlockStmt, patterns []config.ResponseBodyPattern) map[int]responseInfo {
	responses := make(map[int]responseInfo)
	lastStatusCode := 200

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		info := s.getInfoForNode(call.Fun)
		if info == nil {
			return true
		}

		obj := s.getObjectForExpr(call.Fun)
		if obj == nil {
			return true
		}
		funcPath := getFuncPath(obj)

		// Built-in "Magic"
		if funcPath == "net/http.ResponseWriter.WriteHeader" && len(call.Args) == 1 {
			if sc, ok := s.resolveIntValue(call.Args[0]); ok {
				lastStatusCode = sc
				// If this is a 204, log it immediately as it has no body.
				if sc == 204 {
					responses[sc] = responseInfo{Description: "No Content"}
				}
			}
		} else if funcPath == "encoding/json.Encoder.Encode" && len(call.Args) == 1 {
			if tv, ok := info.Types[call.Args[0]]; ok {
				responses[lastStatusCode] = responseInfo{Type: tv.Type}
				lastStatusCode = 200
			}
		}

		// User-configured patterns
		for _, p := range patterns {
			if funcPath == p.FunctionPath {
				statusCode := 200
				var dataArg ast.Expr
				var desc string

				if p.StatusCodeIndex != nil && len(call.Args) > *p.StatusCodeIndex {
					if sc, ok := s.resolveIntValue(call.Args[*p.StatusCodeIndex]); ok {
						statusCode = sc
					}
				}

				if p.DescriptionIndex != nil && len(call.Args) > *p.DescriptionIndex {
					if d, ok := s.resolveStringValue(call.Args[*p.DescriptionIndex]); ok {
						desc = d
					}
				}

				if len(call.Args) > p.DataIndex {
					dataArg = call.Args[p.DataIndex]
				}

				if dataArg != nil {
					if tv, ok := info.Types[dataArg]; ok {
						responses[statusCode] = responseInfo{Type: tv.Type, Description: desc}
					}
				} else {
					responses[statusCode] = responseInfo{Description: desc}
				}
			}
		}
		return true
	})
	return responses
}
