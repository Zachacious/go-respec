package analyzer

import (
	"go/ast"
	"regexp"
	"strings"

	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

var pathParamRegex = regexp.MustCompile(`{([^}]+)}|:(\w+)`)

var httpMethods = map[string]bool{
	"Get": true, "Post": true, "Put": true, "Delete": true, "Patch": true, "Head": true,
}

func (a *Analyzer) processRouteCall(tracker *stateTracker, parentNode *model.RouteNode, call *ast.CallExpr, sel *ast.SelectorExpr, builderCall *ast.CallExpr) {
	methodName := sel.Sel.Name
	// This logic needs to be enhanced by the router definitions from config
	if httpMethods[methodName] {
		a.parseEndpoint(parentNode, call, methodName, builderCall)
	} else if methodName == "Route" || methodName == "Group" {
		a.parseGroup(tracker, parentNode, call, builderCall)
	}
}

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

	matches := pathParamRegex.FindAllStringSubmatch(path, -1)
	for _, match := range matches {
		paramName := match[1]
		if paramName == "" {
			paramName = match[2]
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
	parentNode.Children = append(parentNode.Children, groupNode)

	if len(funcLit.Type.Params.List) > 0 {
		subRouterIdent := funcLit.Type.Params.List[0].Names[0]
		subRouterObj := a.getObjectForExpr(subRouterIdent)
		if subRouterObj != nil {
			tracker.trackedRouters[subRouterObj] = groupNode
			groupNode.GoVar = subRouterObj
			// FIX: Correctly call buildASTVisitor with only one argument.
			ast.Inspect(funcLit.Body, a.buildASTVisitor(tracker))
			delete(tracker.trackedRouters, subRouterObj)
		}
	}
}

func buildFullPath(node *model.RouteNode, suffix string) string {
	if node == nil {
		return suffix
	}
	path := ""
	if node.PathPrefix != "" {
		path = node.PathPrefix + suffix
	} else {
		path = suffix
	}
	return buildFullPath(node.Parent, path)
}
