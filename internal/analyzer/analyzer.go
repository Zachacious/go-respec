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

// stateTracker holds the analysis state during traversal.
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

// Analyze performs the full analysis of the loaded packages using a single unified pass.
func (a *Analyzer) Analyze() (*model.APIModel, error) {
	fmt.Println("Analyzer is now building the route graph...")
	tracker := &stateTracker{
		routeGraph:     &model.RouteNode{},
		trackedRouters: make(map[types.Object]*model.RouteNode),
	}

	// Use a single, unified pass to analyze the entire project.
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

// buildASTVisitor returns the main visitor function for the single analysis pass.
func (a *Analyzer) buildASTVisitor(tracker *stateTracker) func(n ast.Node) bool {
	return func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Case 1: Is this a router initialization?
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

		// Case 2: Is this a method call?
		// FIX: Use the blank identifier `_` for selExpr as it's not used inside the block.
		if _, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
			a.analyzeCallChain(tracker, callExpr)
		}
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

	if receiverObj != nil {
		if node, ok := tracker.trackedRouters[receiverObj]; ok {
			// The receiver is a tracked router. Process the method call.
			a.processRouteCall(tracker, node, call, selExpr, nil)
			return
		}
	}

	// Handle chained calls (e.g., r.With(...).Get(...))
	if nextCall, ok := receiverExpr.(*ast.CallExpr); ok {
		a.analyzeCallChain(tracker, nextCall)
	}
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

// findFuncDecl locates the AST declaration for any function or method.
func (a *Analyzer) findFuncDecl(funcObj types.Object) *ast.FuncDecl {
	if funcObj == nil || funcObj.Pkg() == nil {
		return nil
	}

	var funcPkg *packages.Package
	for _, p := range a.pkgs {
		if p.Types == funcObj.Pkg() {
			funcPkg = p
			break
		}
	}
	if funcPkg == nil {
		return nil
	}

	for _, file := range funcPkg.Syntax {
		if file == nil {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			// Check for both top-level functions and methods on types.
			var defObj types.Object
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				// It's a method. Get the method object from the receiver type.
				recvType := funcPkg.TypesInfo.TypeOf(fn.Recv.List[0].Type)
				if sel, _, _ := types.LookupFieldOrMethod(recvType, true, funcObj.Pkg(), funcObj.Name()); sel != nil {
					defObj = sel
				}
			} else {
				// It's a regular function.
				defObj = funcPkg.TypesInfo.Defs[fn.Name]
			}

			if defObj != nil && defObj.Pos() == funcObj.Pos() {
				return fn
			}
		}
	}
	return nil
}
