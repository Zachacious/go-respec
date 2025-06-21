package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"strconv"

	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

// analyzeHandlers is the entry point for Phase 5.
// It traverses the completed route graph and analyzes each handler.
func (s *State) analyzeHandlers() {
	fmt.Println("Phase 5: Analyzing handlers and generating schemas...")
	s.traverseAndAnalyze(s.RouteGraph)
}

func (s *State) traverseAndAnalyze(node *model.RouteNode) {
	for _, op := range node.Operations {
		s.analyzeHandlerBody(op)
	}
	for _, child := range node.Children {
		s.traverseAndAnalyze(child)
	}
}

// analyzeHandlerBody finds the AST for a handler and inspects its body.
func (s *State) analyzeHandlerBody(op *model.Operation) {
	if op.Spec == nil {
		op.Spec = openapi3.NewOperation()
	}
	// NOTE: openapi3.NewOperation() initializes the Responses field, so we don't use make().

	funcDecl, ok := s.Universe.Functions[op.GoHandler]
	if !ok {
		op.Spec.Summary = op.HandlerName
		op.Spec.AddResponse(200, openapi3.NewResponse().WithDescription("Successful response"))
		return
	}

	if funcDecl.Doc != nil {
		if parsedComment := parseDocComment(funcDecl.Doc); parsedComment != nil {
			op.Spec.Summary = parsedComment.Summary
			op.Spec.Description = parsedComment.Description
			op.Spec.Tags = parsedComment.Tags
		}
	}
	if op.Spec.Summary == "" {
		op.Spec.Summary = op.HandlerName
	}

	reqType := s.findRequestSchema(funcDecl.Body)
	if reqType != nil {
		schemaRef := s.SchemaGen.GenerateSchema(reqType)
		reqBody := openapi3.NewRequestBody().WithContent(
			openapi3.NewContentWithJSONSchemaRef(schemaRef),
		)
		op.Spec.RequestBody = &openapi3.RequestBodyRef{Value: reqBody}
	}

	respType, statusCode := s.findResponseSchema(funcDecl.Body)
	if respType != nil {
		schemaRef := s.SchemaGen.GenerateSchema(respType)
		response := openapi3.NewResponse().WithDescription("Success").WithContent(
			openapi3.NewContentWithJSONSchemaRef(schemaRef),
		)
		op.Spec.AddResponse(statusCode, response)
	}

	// FIX: Use the .Map() method to check the length of the responses.
	if op.Spec.Responses == nil || len(op.Spec.Responses.Map()) == 0 {
		op.Spec.AddResponse(200, openapi3.NewResponse().WithDescription("Successful response"))
	}
}

// findRequestSchema scans a function body for patterns like `json.Decode(&v)`.
func (s *State) findRequestSchema(body *ast.BlockStmt) types.Type {
	var reqType types.Type
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Look for a method call named "Decode"
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Decode" {
			if len(call.Args) == 1 {
				// The argument to Decode is the variable we want the type of.
				arg := call.Args[0]
				info := s.getInfoForNode(arg)
				if info != nil {
					// The type is typically a pointer to the struct, so we get the element type.
					if tv, ok := info.Types[arg]; ok {
						if ptr, isPtr := tv.Type.(*types.Pointer); isPtr {
							reqType = ptr.Elem()
							return false // Stop searching
						}
					}
				}
			}
		}
		return true
	})
	return reqType
}

// findResponseSchema scans for `c.JSON(200, data)` or `json.Encode(data)`.
func (s *State) findResponseSchema(body *ast.BlockStmt) (types.Type, int) {
	var respType types.Type
	statusCode := 200 // Default success code

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// Look for common response methods
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			// Gin-like: c.JSON(200, data)
			if sel.Sel.Name == "JSON" && len(call.Args) == 2 {
				// Get status code from first arg
				if codeLit, ok := call.Args[0].(*ast.BasicLit); ok {
					if sc, err := strconv.Atoi(codeLit.Value); err == nil {
						statusCode = sc
					}
				}
				// Get data type from second arg
				if info := s.getInfoForNode(call.Args[1]); info != nil {
					if tv, ok := info.Types[call.Args[1]]; ok {
						respType = tv.Type
						return false
					}
				}
			}
			// Standard library-like: json.NewEncoder(w).Encode(data)
			if sel.Sel.Name == "Encode" && len(call.Args) == 1 {
				if info := s.getInfoForNode(call.Args[0]); info != nil {
					if tv, ok := info.Types[call.Args[0]]; ok {
						respType = tv.Type
						return false
					}
				}
			}
		}
		return true
	})
	return respType, statusCode
}
