package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/Zachacious/go-respec/internal/model"
	"golang.org/x/tools/go/ast/astutil"
)

func (s *State) performDataFlowAnalysis() {
	fmt.Println("Phase 3: Performing data flow analysis...")
	initialRouterVars := s.findInitialRouterVars()
	fmt.Printf("  [Info] Found %d router initialization sources.\n", len(initialRouterVars))

	var worklist []*types.Var
	for _, v := range initialRouterVars {
		worklist = append(worklist, v)
	}

	for len(worklist) > 0 {
		v := worklist[0]
		worklist = worklist[1:]
		s.findAndProcessUsages(v)
	}
	fmt.Printf("  [Info] Worklist processing complete.\n")
}

func (s *State) findInitialRouterVars() []*types.Var {
	var initialVars []*types.Var
	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
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
								if obj := info.Defs[ident]; obj != nil {
									if v, ok := obj.(*types.Var); ok {
										node := &model.RouteNode{GoVar: v, Parent: s.RouteGraph}
										s.RouteGraph.Children = append(s.RouteGraph.Children, node)
										trackedVal := &TrackedValue{
											Source:    callExpr,
											RouterDef: resolvedType.Definition,
											Node:      node,
										}
										s.VarValues[v] = trackedVal
										initialVars = append(initialVars, v)
									}
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
				parent := path[1]

				if selExpr, ok := parent.(*ast.SelectorExpr); ok {
					if len(path) > 2 {
						if callExpr, ok := path[2].(*ast.CallExpr); ok && callExpr.Fun == selExpr {
							s.processMethodCall(initialValue, callExpr)
						}
					}
				}
				return true
			})
		}
	}
}

func (s *State) processMethodCall(currentValue *TrackedValue, call *ast.CallExpr) {
	selExpr, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	methodName := selExpr.Sel.Name
	routerDef := currentValue.RouterDef

	// Is it an endpoint method? (e.g., .Get, .Post)
	for _, endpointMethod := range routerDef.EndpointMethods {
		if methodName == endpointMethod {
			s.buildRouteFromCall(currentValue, call)
			return
		}
	}

	// Is it a chaining method? (e.g., .With, .Group, .Route)
	var isChain, isGroup bool
	for _, m := range routerDef.GroupMethods {
		if methodName == m {
			// FIX: Correct multi-assignment syntax.
			isChain = true
			isGroup = true
			break
		}
	}
	if !isChain {
		for _, m := range routerDef.MiddlewareWrapperMethods {
			if methodName == m {
				isChain = true
				break
			}
		}
	}

	if isChain {
		pathPrefix := ""
		if isGroup && len(call.Args) > 0 {
			if p, ok := s.resolveStringValue(call.Args[0]); ok {
				pathPrefix = p
			}
		}

		newNode := &model.RouteNode{PathPrefix: pathPrefix, Parent: currentValue.Node}
		currentValue.Node.Children = append(currentValue.Node.Children, newNode)
		newVal := &TrackedValue{
			Source: call, RouterDef: routerDef, Parent: currentValue, PathPrefix: pathPrefix, Node: newNode,
		}

		path, found := s.findPathToNode(call)
		if found && len(path) > 1 {
			parent := path[1]
			if parentSel, ok := parent.(*ast.SelectorExpr); ok {
				if len(path) > 2 {
					if parentCall, ok := path[2].(*ast.CallExpr); ok && parentCall.Fun == parentSel {
						s.processMethodCall(newVal, parentCall)
					}
				}
			}
		}

		for _, arg := range call.Args {
			if funcLit, ok := arg.(*ast.FuncLit); ok {
				if len(funcLit.Type.Params.List) > 0 && len(funcLit.Type.Params.List[0].Names) > 0 {
					paramIdent := funcLit.Type.Params.List[0].Names[0]
					if info := s.getInfoForNode(paramIdent); info != nil {
						if paramObj, ok := info.Defs[paramIdent].(*types.Var); ok {
							s.VarValues[paramObj] = newVal
							s.findAndProcessUsages(paramObj)
						}
					}
				}
			}
		}
	}
}
