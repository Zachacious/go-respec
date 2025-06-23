package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"

	"github.com/Zachacious/go-respec/internal/model"
	"golang.org/x/tools/go/ast/astutil"
)

// performDataFlowAnalysis starts the data flow analysis from initial router variables.
func (s *State) performDataFlowAnalysis() {
	initialRouterVars := s.findInitialRouterVars()
	fmt.Printf(" [Info] Found %d router initialization sources.\n", len(initialRouterVars))

	for _, v := range initialRouterVars {
		s.findAndProcessUsages(v)
	}
	fmt.Printf(" [Info] Worklist processing complete.\n")
}

// findInitialRouterVars finds variables initialized with a configured router type.
func (s *State) findInitialRouterVars() []*types.Var {
	var initialVars []*types.Var
	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
					return true
				}
				callExpr, ok := assign.Rhs[0].(*ast.CallExpr)
				if !ok {
					return true
				}
				info := s.getInfoForNode(callExpr.Fun)
				if info == nil {
					return true
				}

				if sig, ok := info.TypeOf(callExpr.Fun).(*types.Signature); ok {
					if sig.Results().Len() == 1 {
						if resolvedType := s.isResolvedRouterType(sig.Results().At(0).Type()); resolvedType != nil {
							if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
								if obj, ok := info.Defs[ident].(*types.Var); ok {
									node := &model.RouteNode{GoVar: obj, Parent: s.RouteGraph}
									s.RouteGraph.Children = append(s.RouteGraph.Children, node)
									trackedVal := &TrackedValue{
										Source:    callExpr,
										RouterDef: resolvedType.Definition,
										Node:      node,
									}
									s.VarValues[obj] = trackedVal
									initialVars = append(initialVars, obj)
								}
							}
						}
					}
				}
				return true
			})
		}
	}
	return initialVars
}

// findAndProcessUsages finds all usages of a variable `v` and processes them.
func (s *State) findAndProcessUsages(v *types.Var) {
	initialValue, ok := s.VarValues[v]
	if !ok {
		return
	}

	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				ident, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				info := s.getInfoForNode(ident)
				if info == nil || info.Uses[ident] != v {
					return true
				}

				path, _ := astutil.PathEnclosingInterval(file, ident.Pos(), ident.End())
				if len(path) < 2 {
					return true
				}

				if selExpr, ok := path[1].(*ast.SelectorExpr); ok {
					if len(path) > 2 {
						if callExpr, ok := path[2].(*ast.CallExpr); ok && callExpr.Fun == selExpr {
							s.processMethodCall(initialValue, callExpr, file)
							return false
						}
					}
				}
				return true
			})
		}
	}
}

// processMethodCall analyzes a method call on a tracked router value.
func (s *State) processMethodCall(currentValue *TrackedValue, call *ast.CallExpr, file *ast.File) {
	selExpr, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	methodName := selExpr.Sel.Name
	routerDef := currentValue.RouterDef

	// Case 1: Is it an endpoint method? (e.g., .Get, .Post)
	if slices.Contains(routerDef.EndpointMethods, methodName) {
		var handlerFuncDecl *ast.FuncDecl
		if len(call.Args) >= 2 {
			handlerObj := s.getObjectForExpr(call.Args[1])
			if handlerObj != nil {
				handlerFuncDecl = s.Universe.Functions[handlerObj]
			}
		}
		s.buildRouteFromCall(currentValue, call, handlerFuncDecl)
		return
	}

	isGroupMethod := slices.Contains(routerDef.GroupMethods, methodName)
	isMiddlewareMethod := slices.Contains(routerDef.MiddlewareWrapperMethods, methodName)

	// Case 2: Is it a chaining method? (e.g., .With, .Group, .Route)
	if isGroupMethod || isMiddlewareMethod {
		pathPrefix := ""
		if isGroupMethod && len(call.Args) > 0 {
			if p, ok := s.resolveStringValue(call.Args[0]); ok {
				pathPrefix = p
			}
		}

		newNode := &model.RouteNode{PathPrefix: pathPrefix, Parent: currentValue.Node}
		currentValue.Node.Children = append(currentValue.Node.Children, newNode)
		newVal := &TrackedValue{
			Source:     call,
			RouterDef:  routerDef,
			Parent:     currentValue,
			PathPrefix: pathPrefix,
			Node:       newNode,
		}

		// ** THIS BLOCK IS RESTORED **
		// If this is a middleware method, analyze its arguments for security schemes.
		if isMiddlewareMethod {
			for _, arg := range call.Args {
				if middlewareObj := s.getObjectForExpr(arg); middlewareObj != nil {
					inferredSchemes := s.analyzeMiddleware(middlewareObj)
					// Security from middleware applies to the node it's called on.
					currentValue.Node.InferredSecurity = append(currentValue.Node.InferredSecurity, inferredSchemes...)
				}
			}
		}

		// Look for a function literal argument to trace into the new scope.
		for _, arg := range call.Args {
			if funcLit, ok := arg.(*ast.FuncLit); ok {
				if funcLit.Body == nil || len(funcLit.Type.Params.List) == 0 || len(funcLit.Type.Params.List[0].Names) == 0 {
					continue
				}
				paramIdent := funcLit.Type.Params.List[0].Names[0]
				if info := s.getInfoForNode(paramIdent); info != nil {
					if paramObj, ok := info.Defs[paramIdent].(*types.Var); ok {
						s.findAndProcessUsagesInScope(file, funcLit.Body, paramObj, newVal)
					}
				}
			}
		}
	}
}

// findAndProcessUsagesInScope is a correctly scoped analysis function.
func (s *State) findAndProcessUsagesInScope(file *ast.File, scope ast.Node, v *types.Var, trackedVal *TrackedValue) {
	s.VarValues[v] = trackedVal

	ast.Inspect(scope, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		info := s.getInfoForNode(ident)
		if info == nil || info.Uses[ident] != v {
			return true
		}

		path, _ := astutil.PathEnclosingInterval(file, ident.Pos(), ident.End())
		if len(path) < 2 {
			return true
		}
		if selExpr, ok := path[1].(*ast.SelectorExpr); ok {
			if len(path) > 2 {
				if callExpr, ok := path[2].(*ast.CallExpr); ok && callExpr.Fun == selExpr {
					s.processMethodCall(trackedVal, callExpr, file)
					return false
				}
			}
		}
		return true
	})
}
