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

	// --- NEW: Parse Doc Comments ---
	// This is our Level 2 metadata source.
	if parsedComment := parseDocComment(funcDecl.Doc); parsedComment != nil {
		op.Spec.Summary = parsedComment.Summary
		op.Spec.Description = parsedComment.Description
		if len(parsedComment.Tags) > 0 {
			op.Spec.Tags = parsedComment.Tags
		}
	}
	// -------------------------------

	// Inspect the function body for Level 3 inferred data.
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		a.checkForRequestBody(op, callExpr, sg)
		a.checkForResponseBody(op, callExpr, sg)

		// FEAT: Infer Query and Header Parameters
		a.checkForParameters(op, callExpr)

		return true
	})
}

// checkForParameters looks for calls that read query or header values.
func (a *Analyzer) checkForParameters(op *model.Operation, call *ast.CallExpr) {
	selExpr, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return
	}

	// Get the parameter name from the first argument, e.g., the "page" in c.Query("page")
	paramName, ok := a.getStringFromExpr(call.Args[0])
	if !ok {
		return
	}

	paramIn := ""
	methodName := selExpr.Sel.Name

	// Check for Gin-like .Query("name")
	if methodName == "Query" {
		paramIn = openapi3.ParameterInQuery
	}

	// Check for standard library-like r.URL.Query().Get("name")
	// and r.Header.Get("name")
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
		// Avoid adding duplicate parameters if already found in path
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

	// We are looking for methods like Bind, BindJSON, ShouldBindJSON etc.
	if strings.Contains(selExpr.Sel.Name, "Bind") {
		if len(call.Args) > 0 {
			// The argument is the variable to bind to, e.g., &req
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

	// FIX: Use the .Set() method on the Responses struct.
	response := openapi3.NewResponse().WithDescription("Success").WithContent(
		openapi3.NewContentWithJSONSchemaRef(schemaRef),
	)
	op.Spec.Responses.Set(statusCode, &openapi3.ResponseRef{Value: response})
}

// findFuncDecl finds the AST node for a given function object.
func (a *Analyzer) findFuncDecl(funcObj types.Object) *ast.FuncDecl {
	// This implementation is correct. A more robust version would use position info.
	for _, pkg := range a.pkgs {
		for _, file := range pkg.Syntax {
			obj := file.Scope.Lookup(funcObj.Name())
			if obj != nil && obj.Pos() == funcObj.Pos() {
				for _, decl := range file.Decls {
					if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == funcObj.Name() {
						return fn
					}
				}
			}
		}
	}
	return nil
}

// getTypeFromExpr resolves an expression to its underlying type.
func (a *Analyzer) getTypeFromExpr(expr ast.Expr) types.Type {
	info := a.fileTypeInfo[a.currentFile]
	if info != nil {
		if tv, ok := info.Types[expr]; ok {
			return tv.Type
		}
	}
	return nil
}
