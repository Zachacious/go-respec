package analyzer

import (
	"go/ast"
	"go/token"
	"regexp"
	"strconv"
	"strings"

	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

var pathParamRegex = regexp.MustCompile(`{([^}]+)}|:(\w+)`)

// processRouteCall is the main dispatcher for handling method calls on a tracked router.
func (a *Analyzer) processRouteCall(tracker *stateTracker, parentNode *model.RouteNode, call *ast.CallExpr, sel *ast.SelectorExpr, builderCall *ast.CallExpr) {
	methodName := sel.Sel.Name
	receiverType := a.getTypeFromExpr(sel.X)
	if receiverType == nil {
		return
	}

	methodType := a.getRouteMethodType(receiverType, methodName)

	switch methodType {
	case "endpoint":
		a.parseEndpoint(parentNode, call, methodName, builderCall)
	case "group":
		a.parseGroup(tracker, parentNode, call, builderCall)
	case "middleware":
		// This is a middleware wrapper like r.With(...).
		// Its return value is another router, which the recursive analyzeCallChain will handle.
		// No action is needed here, but we recognize it.
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
		param := openapi3.NewPathParameter(paramName).WithSchema(openapi3.NewStringSchema())
		op.Spec.AddParameter(param)
	}

	if builderCall != nil {
		op.BuilderMetadata = a.parseBuilderChain(builderCall)
	}
	node.Operations = append(node.Operations, op)
}

func (a *Analyzer) parseGroup(tracker *stateTracker, parentNode *model.RouteNode, call *ast.CallExpr, builderCall *ast.CallExpr) {
	var pathPrefix string
	var funcLit *ast.FuncLit

	// Handle both r.Route("/prefix", func) and pathless r.Group(func)
	if pathLit, ok := call.Args[0].(*ast.BasicLit); ok {
		pathPrefix, _ = a.getStringFromExpr(pathLit)
		if len(call.Args) > 1 {
			funcLit, _ = call.Args[1].(*ast.FuncLit)
		}
	} else {
		funcLit, _ = call.Args[0].(*ast.FuncLit)
	}
	if funcLit == nil {
		return
	}

	groupNode := &model.RouteNode{PathPrefix: pathPrefix, Parent: parentNode}
	parentNode.Children = append(parentNode.Children, groupNode)

	if len(funcLit.Type.Params.List) > 0 {
		subRouterIdent := funcLit.Type.Params.List[0].Names[0]
		subRouterObj := a.getObjectForExpr(subRouterIdent)
		if subRouterObj != nil {
			tracker.trackedRouters[subRouterObj] = groupNode
			groupNode.GoVar = subRouterObj
			ast.Inspect(funcLit.Body, a.buildASTVisitor(tracker))
			delete(tracker.trackedRouters, subRouterObj)
		}
	}
}

func buildFullPath(node *model.RouteNode, suffix string) string {
	path := suffix
	for n := node; n != nil; n = n.Parent {
		if n.PathPrefix != "" {
			// A simple join logic, needs to be more robust to handle slashes.
			path = n.PathPrefix + path
		}
	}
	return path
}

func (a *Analyzer) getStringFromExpr(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}
