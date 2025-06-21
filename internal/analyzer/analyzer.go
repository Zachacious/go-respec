package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
	"golang.org/x/tools/go/packages"
)

type stateTracker struct {
	routeGraph     *model.RouteNode
	trackedRouters map[types.Object]*model.RouteNode
}

type Analyzer struct {
	projectPath  string
	pkgs         []*packages.Package
	fileTypeInfo map[*ast.File]*types.Info
	currentFile  *ast.File
	routerDefs   []config.RouterDefinition
}

func New(projectPath string, config *config.Config) (*Analyzer, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load packages: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("packages contain errors")
	}
	fileInfoMap := make(map[*ast.File]*types.Info)
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			fileInfoMap[file] = pkg.TypesInfo
		}
	}
	return &Analyzer{
		projectPath:  projectPath,
		pkgs:         pkgs,
		fileTypeInfo: fileInfoMap,
		routerDefs:   config.RouterDefinitions,
	}, nil
}

func (a *Analyzer) Analyze() (*model.APIModel, error) {
	fmt.Println("Analyzer is now building the route graph...")
	tracker := &stateTracker{
		routeGraph:     &model.RouteNode{},
		trackedRouters: make(map[types.Object]*model.RouteNode),
	}
	for _, pkg := range a.pkgs {
		for _, file := range pkg.Syntax {
			a.currentFile = file
			ast.Inspect(file, a.buildASTVisitor(tracker))
		}
	}
	fmt.Println("Route graph built. Analyzing handlers to infer schemas...")
	sg := NewSchemaGenerator()
	a.traverseAndAnalyzeHandlers(tracker.routeGraph, sg)
	apiModel := &model.APIModel{
		RouteGraph: tracker.routeGraph,
	}
	if apiModel.Components == nil {
		apiModel.Components = &openapi3.Components{}
	}
	apiModel.Components.Schemas = sg.schemas
	return apiModel, nil
}

func (a *Analyzer) buildASTVisitor(tracker *stateTracker) func(n ast.Node) bool {
	return func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if a.isRouterInitialization(callExpr) {
			if varObj := a.findAssignStmt(a.currentFile, callExpr); varObj != nil {
				if _, exists := tracker.trackedRouters[varObj]; !exists {
					fmt.Printf("Found root router variable '%s'\n", varObj.Name())
					node := &model.RouteNode{GoVar: varObj}
					if tracker.routeGraph.GoVar == nil {
						tracker.routeGraph = node
					}
					tracker.trackedRouters[varObj] = node
				}
			}
			return true
		}

		if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
			// Trace the receiver of this call all the way back up the chain.
			if parentNode := a.traceReceiverToNode(tracker, selExpr.X); parentNode != nil {
				// We found a chain that ends in a tracked router. Process the call.
				a.processRouteCall(tracker, parentNode, callExpr, selExpr, nil)
			}
		}
		return true
	}
}

// traceReceiverToNode recursively traces a chain of method calls (e.g., r.With(...).With(...))
// until it finds the original tracked router variable, returning its corresponding RouteNode.
func (a *Analyzer) traceReceiverToNode(tracker *stateTracker, expr ast.Expr) *model.RouteNode {
	// Base Case: The expression is a simple variable identifier.
	if ident, ok := expr.(*ast.Ident); ok {
		if obj := a.getObjectForExpr(ident); obj != nil {
			if node, ok := tracker.trackedRouters[obj]; ok {
				return node
			}
		}
	}

	// Recursive Case: The expression is another method call.
	if call, ok := expr.(*ast.CallExpr); ok {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil
		}

		// Check if this method is a middleware wrapper that returns a new router.
		receiverType := a.getTypeFromExpr(sel.X)
		if a.getRouteMethodType(receiverType, sel.Sel.Name) == "middleware" {
			// It is. Continue tracing from ITS receiver (the 'r' in 'r.With(...)').
			return a.traceReceiverToNode(tracker, sel.X)
		}
	}
	return nil
}

func (a *Analyzer) traverseAndAnalyzeHandlers(node *model.RouteNode, sg *SchemaGenerator) {
	if node == nil {
		return
	}
	for _, op := range node.Operations {
		a.analyzeOperation(op, sg)
	}
	for _, child := range node.Children {
		a.traverseAndAnalyzeHandlers(child, sg)
	}
}
