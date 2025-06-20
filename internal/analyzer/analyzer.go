package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
	"golang.org/x/tools/go/packages"
)

// stateTracker holds the analysis state during traversal.
// MOVED to package level.
type stateTracker struct {
	routeGraph     *model.RouteNode
	trackedRouters map[types.Object]*model.RouteNode
}

// Analyzer holds the state for a single analysis run.
type Analyzer struct {
	projectPath  string
	pkgs         []*packages.Package
	fileTypeInfo map[*ast.File]*types.Info
	currentFile  *ast.File
	routerDefs   []config.RouterDefinition
}

// New creates and initializes a new Analyzer for the given project path.
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

// Analyze performs the full analysis of the loaded packages.
func (a *Analyzer) Analyze() (*model.APIModel, error) {
	fmt.Println("Analyzer is now building the route graph...")
	tracker := &stateTracker{
		routeGraph:     &model.RouteNode{},
		trackedRouters: make(map[types.Object]*model.RouteNode),
	}

	for _, pkg := range a.pkgs {
		for _, file := range pkg.Syntax {
			a.currentFile = file
			ast.Inspect(file, func(n ast.Node) bool {
				if callExpr, ok := n.(*ast.CallExpr); ok {
					// Start analysis from any function call.
					a.analyzeCallChain(tracker, callExpr)
				}
				return true
			})
		}
	}

	fmt.Println("Root routers identified. Now parsing routes...")
	for _, pkg := range a.pkgs {
		for _, file := range pkg.Syntax {
			a.currentFile = file
			ast.Inspect(file, a.buildASTVisitor(tracker, tracker.routeGraph))
		}
	}

	fmt.Println("Route graph complete. Analyzing handlers to infer schemas...")
	sg := NewSchemaGenerator()
	a.traverseAndAnalyzeHandlers(tracker.routeGraph, sg)

	apiModel := &model.APIModel{
		RouteGraph: tracker.routeGraph,
	}
	apiModel.Components.Schemas = sg.schemas

	return apiModel, nil
}

// traverseAndAnalyzeHandlers is a recursive function to walk the graph and analyze each operation.
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

// findRootRouters returns a visitor function that only looks for router initializations.
func (a *Analyzer) findRootRouters(tracker *stateTracker) func(n ast.Node) bool {
	return func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if a.isRouterInitialization(callExpr) {
			_, varObj := a.findAssignStmt(a.currentFile, callExpr)
			if varObj != nil {
				fmt.Printf("Found root router variable '%s' in %s\n",
					varObj.Name(), a.pkgs[0].Fset.File(n.Pos()).Name())
				node := &model.RouteNode{GoVar: varObj}
				tracker.routeGraph = node
				tracker.trackedRouters[varObj] = node
			}
		}
		return true
	}
}

// buildASTVisitor returns a visitor function that finds route registrations.
// func (a *Analyzer) buildASTVisitor(tracker *stateTracker, currentNode *model.RouteNode) func(n ast.Node) bool {
// 	return func(n ast.Node) bool {
// 		callExpr, ok := n.(*ast.CallExpr)
// 		if !ok {
// 			return true
// 		}
// 		selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
// 		if !ok {
// 			return true
// 		}

// 		// Check for the Fluent Builder first (e.g., respec.Route(r.Get(...)))
// 		if a.isRespecCall(selExpr) {
// 			if len(callExpr.Args) > 0 {
// 				if innerCall, ok := callExpr.Args[0].(*ast.CallExpr); ok {
// 					if innerSel, ok := innerCall.Fun.(*ast.SelectorExpr); ok {
// 						if ident, ok := innerSel.X.(*ast.Ident); ok {
// 							if varObj := a.getObjectForExpr(ident); varObj != nil {
// 								if node, ok := tracker.trackedRouters[varObj]; ok {
// 									// Pass the outer builder call for metadata parsing
// 									a.processRouteCall(tracker, node, innerCall, innerSel, callExpr)
// 								}
// 							}
// 						}
// 					}
// 				}
// 			}
// 			return false // We've processed the entire chain, don't re-process inside
// 		}

// 		// Fallback for raw route calls not wrapped by the builder
// 		if ident, ok := selExpr.X.(*ast.Ident); ok {
// 			if varObj := a.getObjectForExpr(ident); varObj != nil {
// 				if node, ok := tracker.trackedRouters[varObj]; ok {
// 					// Pass nil for the builder call
// 					a.processRouteCall(tracker, node, callExpr, selExpr, nil)
// 				}
// 			}
// 		}
// 		return true
// 	}
// }

func (a *Analyzer) buildASTVisitor(tracker *stateTracker, currentNode *model.RouteNode) func(n ast.Node) bool {
	return func(n ast.Node) bool {
		// We only care about function calls.
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Recursively analyze the call chain to find the root variable.
		a.analyzeCallChain(tracker, callExpr)

		return true
	}
}

// analyzeCallChain is the recursive engine for router tracking.
func (a *Analyzer) analyzeCallChain(tracker *stateTracker, call *ast.CallExpr) {
	selExpr, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	receiverExpr := selExpr.X
	receiverObj := a.getObjectForExpr(receiverExpr)

	// If the receiver is a tracked router, process the call.
	if receiverObj != nil {
		if node, ok := tracker.trackedRouters[receiverObj]; ok {
			a.processRouteCall(tracker, node, call, selExpr, nil)
			return
		}
	}

	// If not, check if the receiver is a chained call (e.g., r.With(...))
	if nextCall, ok := receiverExpr.(*ast.CallExpr); ok {
		retType := a.getTypeFromExpr(nextCall)
		if retType != nil && a.isRouterType(retType) {
			// This is a middleware wrapper. Get the original router it was called on.
			originalRouterObj := a.getObjectForExpr(nextCall.Fun.(*ast.SelectorExpr).X)
			if originalRouterNode, ok := tracker.trackedRouters[originalRouterObj]; ok {
				// The result of this call is a new, temporary router.
				// We create a new tracker to track it, associated with the original node.
				newTracker := tracker.Clone()
				newTracker.trackedRouters[a.getObjectForExpr(nextCall)] = originalRouterNode
				a.analyzeCallChain(newTracker, call) // Recurse with the new tracker
				return
			}
		}
		// Recurse up the chain if it's not a router-returning call.
		a.analyzeCallChain(tracker, nextCall)
	}
}

// isRespecRouteCall checks if a selector expression is a call to `respec.Route`.
func (a *Analyzer) isRespecCall(sel *ast.SelectorExpr) bool {
	if pkgIdent, ok := sel.X.(*ast.Ident); ok {
		// A more robust check would use type info to get the package path.
		return pkgIdent.Name == "respec" && (sel.Sel.Name == "Route" || sel.Sel.Name == "Group")
	}
	return false
}

func (t *stateTracker) Clone() *stateTracker {
	newTracker := &stateTracker{
		routeGraph:     t.routeGraph,
		trackedRouters: make(map[types.Object]*model.RouteNode, len(t.trackedRouters)),
	}
	for k, v := range t.trackedRouters {
		newTracker.trackedRouters[k] = v
	}
	return newTracker
}
