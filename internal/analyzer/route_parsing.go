package analyzer

import (
	"go/ast"
	"regexp"
	"strings"

	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

var httpMethods = map[string]bool{
	"Get":    true,
	"Post":   true,
	"Put":    true,
	"Delete": true,
	"Patch":  true,
	"Head":   true,
}

var pathParamRegex = regexp.MustCompile(`{([^}]+)}|:(\w+)`)

// processRouteCall is the main dispatcher. It's called when we find a method call on a router we're tracking.
func (a *Analyzer) processRouteCall(tracker *stateTracker, parentNode *model.RouteNode, call *ast.CallExpr, sel *ast.SelectorExpr, builderCall *ast.CallExpr) {
	methodName := sel.Sel.Name
	if httpMethods[methodName] {
		a.parseEndpoint(parentNode, call, methodName, builderCall)
	} else if methodName == "Route" || methodName == "Group" {
		a.parseGroup(tracker, parentNode, call, builderCall)
	}
}

// parseEndpoint handles a terminal route registration like r.Get("/users", GetUsers)
func (a *Analyzer) parseEndpoint(node *model.RouteNode, call *ast.CallExpr, httpMethod string, builderCall *ast.CallExpr) {
	if len(call.Args) < 2 {
		return
	}
	path, ok := a.getStringFromExpr(call.Args[0])
	if !ok {
		return
	}

	handlerObj := a.getObjectForExpr(call.Args[1])
	if handlerObj == nil {
		return
	}

	op := &model.Operation{
		HTTPMethod:     strings.ToUpper(httpMethod),
		FullPath:       buildFullPath(node, path),
		HandlerPackage: handlerObj.Pkg().Name(),
		HandlerName:    handlerObj.Name(),
		GoHandler:      handlerObj,
		Spec:           openapi3.NewOperation(),
	}

	// FEAT: Infer Path Parameters from URL string
	matches := pathParamRegex.FindAllStringSubmatch(path, -1)
	for _, match := range matches {
		paramName := match[1] // For {id} style
		if paramName == "" {
			paramName = match[2] // For :id style
		}

		param := openapi3.NewPathParameter(paramName).
			WithSchema(openapi3.NewStringSchema())
		op.Spec.AddParameter(param)
	}

	if builderCall != nil {
		op.BuilderMetadata = a.parseBuilderChain(builderCall)
	}

	node.Operations = append(node.Operations, op)
}

// parseGroup handles a grouping function like r.Route("/v1", func(r chi.Router) { ... })
func (a *Analyzer) parseGroup(tracker *stateTracker, parentNode *model.RouteNode, call *ast.CallExpr, builderCall *ast.CallExpr) {
	if len(call.Args) < 2 {
		return
	}
	pathPrefix, _ := a.getStringFromExpr(call.Args[0])
	funcLit, ok := call.Args[1].(*ast.FuncLit)
	if !ok {
		return
	}

	groupNode := &model.RouteNode{
		PathPrefix: pathPrefix,
		Parent:     parentNode,
	}
	if builderCall != nil {
		// We can attach builder metadata to groups as well if we extend the model
	}
	parentNode.Children = append(parentNode.Children, groupNode)

	if len(funcLit.Type.Params.List) > 0 {
		subRouterIdent := funcLit.Type.Params.List[0].Names[0]
		subRouterObj := a.fileTypeInfo[a.currentFile].Defs[subRouterIdent]
		if subRouterObj != nil {
			tracker.trackedRouters[subRouterObj] = groupNode
			groupNode.GoVar = subRouterObj
			ast.Inspect(funcLit.Body, a.buildASTVisitor(tracker, groupNode))
			delete(tracker.trackedRouters, subRouterObj)
		}
	}
}

// buildFullPath walks up the RouteGraph to construct the full path for an endpoint.
func buildFullPath(node *model.RouteNode, suffix string) string {
	if node == nil {
		return suffix
	}
	// A simple join logic, needs to be more robust to handle slashes.
	if node.PathPrefix != "" {
		return buildFullPath(node.Parent, node.PathPrefix+suffix)
	}
	return buildFullPath(node.Parent, suffix)
}
