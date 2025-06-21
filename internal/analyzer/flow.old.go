package analyzer

// import (
// 	"fmt"
// 	"go/ast"
// 	"go/types"

// 	"github.com/Zachacious/go-respec/internal/model"
// 	"golang.org/x/tools/go/ast/astutil"
// )

// // performDataFlowAnalysis is the new, more robust Phase 3 entry point.
// func (s *State) performDataFlowAnalysis() {
// 	fmt.Println("Phase 3: Performing data flow analysis...")
// 	// Find the initial set of variables that are routers.
// 	initialRouterVars := s.findInitialRouterVars()
// 	fmt.Printf("  [Info] Found %d router initialization sources.\n", len(initialRouterVars))

// 	// The worklist now contains `types.Object`s, which represent variables.
// 	var worklist []*types.Var
// 	for _, v := range initialRouterVars {
// 		worklist = append(worklist, v)
// 	}

// 	// Process items from the worklist until it's empty.
// 	for len(worklist) > 0 {
// 		v := worklist[0]
// 		worklist = worklist[1:]

// 		// For each variable, find all its usages and process them.
// 		s.findAndProcessUsages(v)
// 	}
// 	fmt.Printf("  [Info] Worklist processing complete.\n")
// }

// // findInitialRouterVars finds all variables that are initialized with a new router instance.
// func (s *State) findInitialRouterVars() []*types.Var {
// 	var initialVars []*types.Var
// 	for _, pkg := range s.pkgs {
// 		for _, file := range pkg.Syntax {
// 			ast.Inspect(file, func(n ast.Node) bool {
// 				assign, ok := n.(*ast.AssignStmt)
// 				if !ok {
// 					return true
// 				}

// 				// Looking for `var := someFunc()`
// 				if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
// 					return true
// 				}

// 				callExpr, ok := assign.Rhs[0].(*ast.CallExpr)
// 				if !ok {
// 					return true
// 				}

// 				// Check if the function call returns a known router type.
// 				info := s.getInfoForNode(callExpr.Fun)
// 				if info == nil {
// 					return true
// 				}

// 				if sig, ok := info.TypeOf(callExpr.Fun).(*types.Signature); ok {
// 					if sig.Results().Len() == 1 {
// 						if resolvedType := s.isResolvedRouterType(sig.Results().At(0).Type()); resolvedType != nil {
// 							// This is a router initialization. Get the variable it's assigned to.
// 							if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
// 								if obj := info.Defs[ident]; obj != nil {
// 									if v, ok := obj.(*types.Var); ok {
// 										// We found an initial router variable. Track it.
// 										node := &model.RouteNode{GoVar: v, Parent: s.RouteGraph}
// 										s.RouteGraph.Children = append(s.RouteGraph.Children, node)
// 										trackedVal := &TrackedValue{
// 											Source:    callExpr,
// 											RouterDef: resolvedType.Definition,
// 											Node:      node,
// 										}
// 										s.VarValues[v] = trackedVal
// 										initialVars = append(initialVars, v)
// 									}
// 								}
// 							}
// 						}
// 					}
// 				}
// 				return true
// 			})
// 		}
// 	}
// 	return initialVars
// }

// // findAndProcessUsages finds all references to a given variable object and processes them.
// func (s *State) findAndProcessUsages(v *types.Var) {
// 	initialValue, ok := s.VarValues[v]
// 	if !ok {
// 		return
// 	}

// 	for _, pkg := range s.pkgs {
// 		for _, file := range pkg.Syntax {
// 			ast.Inspect(file, func(n ast.Node) bool {
// 				ident, ok := n.(*ast.Ident)
// 				if !ok {
// 					return true
// 				}

// 				// Is this identifier a usage of our tracked variable?
// 				info := s.getInfoForNode(ident)
// 				if info == nil || info.Uses[ident] != v {
// 					return true
// 				}

// 				// We found a usage. Find its parent to see how it's used.
// 				path, _ := astutil.PathEnclosingInterval(file, ident.Pos(), ident.End())
// 				if len(path) < 2 {
// 					return true
// 				}
// 				parent := path[1] // The immediate parent of the identifier.

// 				// Is the variable being used as the receiver of a method call? (e.g., `r`.Get)
// 				if selExpr, ok := parent.(*ast.SelectorExpr); ok {
// 					if len(path) > 2 {
// 						if callExpr, ok := path[2].(*ast.CallExpr); ok && callExpr.Fun == selExpr {
// 							s.processMethodCall(initialValue, callExpr)
// 						}
// 					}
// 				}
// 				return true
// 			})
// 		}
// 	}
// }

// // processMethodCall handles a method called on a tracked router variable.
// func (s *State) processMethodCall(currentValue *TrackedValue, call *ast.CallExpr) {
// 	selExpr := call.Fun.(*ast.SelectorExpr)
// 	methodName := selExpr.Sel.Name
// 	routerDef := currentValue.RouterDef

// 	// Is it an endpoint method?
// 	for _, endpointMethod := range routerDef.EndpointMethods {
// 		if methodName == endpointMethod {
// 			// FIX: Pass currentValue, which is a *TrackedValue, to match the new signature.
// 			s.buildRouteFromCall(currentValue, call)
// 			return
// 		}
// 	}

// 	// Is it a chaining method (group or middleware)?
// 	// FIX: Correctly declare boolean variables.
// 	isChain := false
// 	isGroup := false
// 	for _, m := range routerDef.GroupMethods {
// 		if methodName == m {
// 			isChain, isGroup = true, true
// 			break
// 		}
// 	}
// 	if !isChain {
// 		for _, m := range routerDef.MiddlewareWrapperMethods {
// 			if methodName == m {
// 				isChain = true
// 				break
// 			}
// 		}
// 	}

// 	if isChain {
// 		pathPrefix := ""
// 		if isGroup && len(call.Args) > 0 {
// 			if p, ok := s.resolveStringValue(call.Args[0]); ok {
// 				pathPrefix = p
// 			}
// 		}

// 		newNode := &model.RouteNode{PathPrefix: pathPrefix, Parent: currentValue.Node}
// 		currentValue.Node.Children = append(currentValue.Node.Children, newNode)
// 		newVal := &TrackedValue{
// 			Source: call, RouterDef: routerDef, Parent: currentValue, PathPrefix: pathPrefix, Node: newNode,
// 		}

// 		// --- START OF FIX ---
// 		// The result of a chaining call (like `r.With(...)`) is not assigned to a variable.
// 		// It is used immediately as the receiver of the *next* call (e.g., `.Post(...)`).
// 		// We must find this "parent" call and process it using our new tracked value.
// 		path, _ := s.findPathToNode(call)
// 		if len(path) > 2 {
// 			// The parent of a `CallExpr` in a chain is a `SelectorExpr` (e.g., `.Post`).
// 			// The grandparent is the `CallExpr` for that selector (e.g., `...Post(...)`).
// 			if parentSel, ok := path[len(path)-2].(*ast.SelectorExpr); ok {
// 				if parentCall, ok := path[len(path)-3].(*ast.CallExpr); ok && parentCall.Fun == parentSel {
// 					// We found the next call in the chain. Process it now with the new value.
// 					s.processMethodCall(newVal, parentCall)
// 				}
// 			}
// 		}
// 		// --- END OF FIX ---

// 		for _, arg := range call.Args {
// 			if funcLit, ok := arg.(*ast.FuncLit); ok {
// 				if len(funcLit.Type.Params.List) > 0 && len(funcLit.Type.Params.List[0].Names) > 0 {
// 					paramIdent := funcLit.Type.Params.List[0].Names[0]
// 					if info := s.getInfoForNode(paramIdent); info != nil {
// 						if paramObj, ok := info.Defs[paramIdent].(*types.Var); ok {
// 							s.VarValues[paramObj] = newVal
// 							s.findAndProcessUsages(paramObj) // Recurse immediately
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}
// }

// // // performDataFlowAnalysis is Phase 3...
// // func (s *State) performDataFlowAnalysis() {
// // 	fmt.Println("Phase 3: Performing data flow analysis...")
// // 	s.findSources()
// // 	fmt.Printf("  [Info] Found %d router initialization sources. Starting worklist processing.\n", len(s.Worklist))

// // 	// Process items from the worklist until it's empty.
// // 	for len(s.Worklist) > 0 {
// // 		item := s.Worklist[0]
// // 		s.Worklist = s.Worklist[1:]

// // 		// Prevent processing the same node twice.
// // 		if s.processed[item.Node] {
// // 			continue
// // 		}
// // 		s.processed[item.Node] = true

// // 		s.traceWorklistItem(item)
// // 	}

// // 	fmt.Printf("  [Info] Worklist processing complete.\n")
// // }

// // traceWorklistItem is the main dispatcher for the analysis of a single worklist item.
// // func (s *State) traceWorklistItem(item WorklistItem) {
// // 	path, _ := s.findPathToNode(item.Node)
// // 	if path == nil || len(path) < 2 {
// // 		return
// // 	}

// // 	parent := path[len(path)-2]

// // 	// The logic depends on what the tracked node IS, and what its parent is.
// // 	switch node := item.Node.(type) {
// // 	case *ast.Ident:
// // 		// The tracked node is a variable. How is it being used?
// // 		switch p := parent.(type) {
// // 		case *ast.SelectorExpr:
// // 			// Usage: variable.Method(). Check if the grandparent is a CallExpr.
// // 			if len(path) > 2 {
// // 				if call, ok := path[len(path)-3].(*ast.CallExpr); ok && call.Fun == p {
// // 					s.traceMethodCall(item, call)
// // 				}
// // 			}
// // 		case *ast.AssignStmt:
// // 			// Assignment: newVar := variable
// // 			s.traceAssignment(item, p)
// // 		case *ast.CallExpr:
// // 			// Argument: someFunc(variable)
// // 			for i, arg := range p.Args {
// // 				if arg == node {
// // 					s.traceFunctionArgument(item, p, i)
// // 					break
// // 				}
// // 			}
// // 		}

// // 	case *ast.CallExpr:
// // 		// The tracked node is the RESULT of a call, e.g., (r.With(...)).Post(...)
// // 		// How is this result being used?
// // 		if p, ok := parent.(*ast.SelectorExpr); ok {
// // 			// The result is used as the receiver of another method call.
// // 			if len(path) > 2 {
// // 				if call, ok := path[len(path)-3].(*ast.CallExpr); ok && call.Fun == p {
// // 					// This is the chained call. Trace it.
// // 					s.traceMethodCall(item, call)
// // 				}
// // 			}
// // 		}
// // 	}
// // }

// // getInfoForNode is a helper to find the correct types.Info for any given node.
// func (s *State) getInfoForNode(node ast.Node) *types.Info {
// 	for _, pkg := range s.pkgs {
// 		for _, file := range pkg.Syntax {
// 			if file.Pos() <= node.Pos() && node.End() <= file.End() {
// 				return s.fileTypeInfo[file]
// 			}
// 		}
// 	}
// 	return nil
// }

// // ... (the rest of flow.go: findSources, isResolvedRouterType, etc.)

// // findSources scans the entire AST for calls to known router initializers
// // and adds them as the initial items to the worklist.
// func (s *State) findSources() {
// 	for _, pkg := range s.pkgs {
// 		for _, file := range pkg.Syntax {
// 			info := s.fileTypeInfo[file]
// 			if info == nil {
// 				continue
// 			}

// 			ast.Inspect(file, func(n ast.Node) bool {
// 				callExpr, ok := n.(*ast.CallExpr)
// 				if !ok {
// 					return true // Continue traversal
// 				}

// 				// Is this call producing a value of a type we care about?
// 				obj := info.Uses[getIdentifier(callExpr.Fun)]
// 				if fn, isFunc := obj.(*types.Func); isFunc {
// 					sig := fn.Signature()
// 					// Check if the function returns a single value
// 					if sig.Results().Len() == 1 {
// 						retType := sig.Results().At(0).Type()
// 						if resolvedType := s.isResolvedRouterType(retType); resolvedType != nil {
// 							// SOURCE FOUND! This is a call to a router initializer.
// 							// This is the root of a tracking chain.
// 							node := &model.RouteNode{GoVar: nil, Parent: s.RouteGraph} // Create a graph node
// 							s.RouteGraph.Children = append(s.RouteGraph.Children, node)

// 							trackedVal := &TrackedValue{
// 								Source:    callExpr,
// 								RouterDef: resolvedType.Definition,
// 								Node:      node, // Associate the graph node
// 							}
// 							// Link the expression result to this new value.
// 							s.ExprResults[callExpr] = trackedVal

// 							// Add this call expression to the worklist to be analyzed.
// 							s.Worklist = append(s.Worklist, WorklistItem{
// 								Node:  callExpr,
// 								Value: trackedVal,
// 							})
// 						}
// 					}
// 				}
// 				return true
// 			})
// 		}
// 	}
// }

// // isResolvedRouterType checks if a given type 't' matches one of the router types
// // we resolved in Phase 1. It returns the corresponding ResolvedType if it's a match.
// func (s *State) isResolvedRouterType(t types.Type) *ResolvedType {
// 	var named *types.Named
// 	if ptr, isPtr := t.(*types.Pointer); isPtr {
// 		if n, isNamed := ptr.Elem().(*types.Named); isNamed {
// 			named = n
// 		}
// 	} else if n, isNamed := t.(*types.Named); isNamed {
// 		named = n
// 	}

// 	if named == nil {
// 		return nil
// 	}

// 	// Check against all the types we resolved from the config.
// 	for _, resolvedType := range s.ResolvedRouterTypes {
// 		if named.Obj() == resolvedType.Object.Obj() {
// 			return resolvedType
// 		}
// 	}
// 	return nil
// }

// // getIdentifier is a simple helper to get the *ast.Ident from a call's Fun expression.
// func getIdentifier(expr ast.Expr) *ast.Ident {
// 	switch e := expr.(type) {
// 	case *ast.Ident:
// 		return e
// 	case *ast.SelectorExpr:
// 		return e.Sel
// 	}
// 	return nil
// }

// // traceFunctionArgument handles the case where a tracked value is passed as an
// // argument to a function call. It traces the flow into that function.
// func (s *State) traceFunctionArgument(item WorklistItem, call *ast.CallExpr, argIndex int) {
// 	// 1. Find the function being called.
// 	calleeObj := s.getObjectForExpr(call.Fun)
// 	if calleeObj == nil {
// 		return
// 	}

// 	// 2. Look up the function's declaration in our universe.
// 	funcDecl, ok := s.Universe.Functions[calleeObj]
// 	if !ok || funcDecl.Type == nil || funcDecl.Type.Params == nil || funcDecl.Body == nil {
// 		return // We can't analyze this function.
// 	}

// 	// 3. Find the parameter object corresponding to our argument.
// 	if len(funcDecl.Type.Params.List) <= argIndex {
// 		return
// 	}
// 	paramField := funcDecl.Type.Params.List[argIndex]
// 	if len(paramField.Names) == 0 {
// 		return
// 	}
// 	paramIdent := paramField.Names[0]
// 	info := s.getInfoForNode(paramIdent)
// 	paramObj := info.Defs[paramIdent]
// 	if paramObj == nil {
// 		return
// 	}

// 	// 4. The parameter inside the function now holds our tracked value.
// 	fmt.Printf("  [Flow] Tracing value into function '%s' via parameter '%s'\n", calleeObj.Name(), paramObj.Name())
// 	s.VarValues[paramObj] = item.Value

// 	// 5. Find all usages of THAT PARAMETER within the function's body and queue them.
// 	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
// 		if ident, ok := n.(*ast.Ident); ok {
// 			// Check if this identifier is a usage of our parameter
// 			if usageInfo := s.getInfoForNode(ident); usageInfo != nil && usageInfo.Uses[ident] == paramObj {
// 				s.Worklist = append(s.Worklist, WorklistItem{
// 					Node:  ident,
// 					Value: item.Value, // Pass the same value down
// 				})
// 			}
// 		}
// 		return true
// 	})
// }
