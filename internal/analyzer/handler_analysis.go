package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

// analyzeOperation finds the handler's source code and inspects it.
func (a *Analyzer) analyzeOperation(op *model.Operation, sg *SchemaGenerator) {
	if op.GoHandler == nil {
		return
	}

	funcDecl := a.findFuncDecl(op.GoHandler)
	if funcDecl == nil {
		fmt.Printf("Could not find source for handler %s\n", op.HandlerName)
		return
	}

	if parsedComment := parseDocComment(funcDecl.Doc); parsedComment != nil {
		op.Spec.Summary = parsedComment.Summary
		op.Spec.Description = parsedComment.Description
		if len(parsedComment.Tags) > 0 {
			op.Spec.Tags = parsedComment.Tags
		}
	}

	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		a.checkForRequestBody(op, callExpr, sg)
		a.checkForResponseBody(op, callExpr, sg)
		a.checkForParameters(op, callExpr)
		return true
	})
}

// findFuncDecl locates the AST declaration for any function or method by its position.
func (a *Analyzer) findFuncDecl(funcObj types.Object) *ast.FuncDecl {
	if funcObj == nil {
		return nil
	}
	pos := funcObj.Pos()

	// Find which of the loaded packages contains the declaration.
	for _, pkg := range a.pkgs {
		for _, file := range pkg.Syntax {
			// Check if the declaration's position is within this file.
			if file.Pos() <= pos && pos < file.End() {
				var foundDecl *ast.FuncDecl
				ast.Inspect(file, func(n ast.Node) bool {
					// We are looking for a function declaration AST node.
					fn, ok := n.(*ast.FuncDecl)
					if !ok {
						return true // Continue searching
					}

					// Get the types.Object for the function name identifier in the AST.
					var defObj types.Object
					if fn.Recv != nil && len(fn.Recv.List) > 0 {
						// This is a method. We must look up the method on the receiver's type.
						recvType := pkg.TypesInfo.TypeOf(fn.Recv.List[0].Type)
						if method, _, _ := types.LookupFieldOrMethod(recvType, true, pkg.Types, fn.Name.Name); method != nil {
							defObj = method
						}
					} else {
						// This is a regular function.
						defObj = pkg.TypesInfo.Defs[fn.Name]
					}

					// If the AST node's object position matches our target object's position, we have found it.
					if defObj != nil && defObj.Pos() == pos {
						foundDecl = fn
						return false // Stop searching
					}
					return true
				})
				return foundDecl
			}
		}
	}
	return nil
}

// checkForParameters looks for calls that read query or header values.
func (a *Analyzer) checkForParameters(op *model.Operation, call *ast.CallExpr) {
	selExpr, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return
	}

	paramName, ok := a.getStringFromExpr(call.Args[0])
	if !ok {
		return
	}

	paramIn := ""
	methodName := selExpr.Sel.Name

	if methodName == "Query" {
		paramIn = openapi3.ParameterInQuery
	}
	if methodName == "Get" {
		if receiverSel, ok := selExpr.X.(*ast.CallExpr); ok {
			if innerSel, ok := receiverSel.Fun.(*ast.SelectorExpr); ok && innerSel.Sel.Name == "Query" {
				paramIn = openapi3.ParameterInQuery
			}
		} else if receiverSel, ok := selExpr.X.(*ast.SelectorExpr); ok && receiverSel.Sel.Name == "Header" {
			paramIn = openapi3.ParameterInHeader
		}
	}

	if paramIn != "" {
		for _, p := range op.Spec.Parameters {
			if p.Value != nil && p.Value.Name == paramName {
				return
			}
		}
		param := &openapi3.Parameter{Name: paramName, In: paramIn}
		param.Schema = openapi3.NewSchemaRef("", openapi3.NewStringSchema())
		op.Spec.AddParameter(param)
	}
}

// checkForRequestBody looks for calls like c.BindJSON(&req)
func (a *Analyzer) checkForRequestBody(op *model.Operation, call *ast.CallExpr, sg *SchemaGenerator) {
	selExpr, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	if strings.Contains(selExpr.Sel.Name, "Bind") {
		if len(call.Args) > 0 {
			arg := call.Args[0]
			varObj := a.getTypeFromExpr(arg)
			if varObj != nil {
				schemaRef := sg.GenerateSchemaRef(varObj)
				op.Spec.RequestBody = &openapi3.RequestBodyRef{
					Value: openapi3.NewRequestBody().WithContent(
						openapi3.NewContentWithJSONSchemaRef(schemaRef),
					),
				}
			}
		}
	}
}

// checkForResponseBody looks for calls like c.JSON(200, data)
func (a *Analyzer) checkForResponseBody(op *model.Operation, call *ast.CallExpr, sg *SchemaGenerator) {
	selExpr, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selExpr.Sel.Name != "JSON" || len(call.Args) != 2 {
		return
	}
	codeLit, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return
	}
	statusCode := codeLit.Value
	dataObj := a.getTypeFromExpr(call.Args[1])
	if dataObj == nil {
		return
	}

	schemaRef := sg.GenerateSchemaRef(dataObj)
	if op.Spec.Responses == nil {
		op.Spec.Responses = openapi3.NewResponses()
	}

	response := openapi3.NewResponse().WithDescription("Success").WithContent(
		openapi3.NewContentWithJSONSchemaRef(schemaRef),
	)
	op.Spec.Responses.Set(statusCode, &openapi3.ResponseRef{Value: response})
}

// getTypeFromExpr resolves an expression to its underlying type.
func (a *Analyzer) getTypeFromExpr(expr ast.Expr) types.Type {
	if a.fileTypeInfo != nil {
		if info, ok := a.fileTypeInfo[a.currentFile]; ok && info != nil {
			if tv, ok := info.Types[expr]; ok {
				return tv.Type
			}
		}
	}
	return nil
}
