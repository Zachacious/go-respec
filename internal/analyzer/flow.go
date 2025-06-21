package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/Zachacious/go-respec/internal/model"
	"golang.org/x/tools/go/ast/astutil"
)

// performDataFlowAnalysis is Phase 3...
func (s *State) performDataFlowAnalysis() {
	fmt.Println("Phase 3: Performing data flow analysis...")
	s.findSources()
	fmt.Printf("  [Info] Found %d router initialization sources. Starting worklist processing.\n", len(s.Worklist))

	// Process items from the worklist until it's empty.
	for len(s.Worklist) > 0 {
		item := s.Worklist[0]
		s.Worklist = s.Worklist[1:]

		// Prevent processing the same node twice.
		if s.processed[item.Node] {
			continue
		}
		s.processed[item.Node] = true

		s.traceWorklistItem(item)
	}

	fmt.Printf("  [Info] Worklist processing complete.\n")
}

// traceWorklistItem is the main dispatcher for the analysis of a single worklist item.
func (s *State) traceWorklistItem(item WorklistItem) {
	// Find the path from the AST root to our current node. This is crucial for finding the parent.
	path, _ := s.findPathToNode(item.Node)
	if path == nil {
		return // Should not happen if node is from a file we have
	}

	// The parent of the node tells us its context.
	parent := path[len(path)-2]

	switch p := parent.(type) {
	case *ast.AssignStmt:
		s.traceAssignment(item, p)
	case *ast.CallExpr:
		// Is our node the receiver of a method call? (e.g., the `r` in `r.Get(...)`)
		if sel, ok := p.Fun.(*ast.SelectorExpr); ok && sel.X == item.Node {
			s.traceMethodCall(item, p)
			return
		}

		// Is our node being passed as an argument to a function?
		for i, arg := range p.Args {
			if arg == item.Node {
				s.traceFunctionArgument(item, p, i)
				break
			}
		}
	}
}

// traceAssignment handles cases where a tracked value is assigned to a variable.
func (s *State) traceAssignment(item WorklistItem, assign *ast.AssignStmt) {
	// We are interested in `var := value` or `var = value`.
	// The item.Node is on the right-hand side.
	for i, rhsExpr := range assign.Rhs {
		if rhsExpr != item.Node {
			continue
		}

		if i >= len(assign.Lhs) {
			continue
		}
		lhs := assign.Lhs[i]

		// Get the variable object being assigned to.
		if ident, ok := lhs.(*ast.Ident); ok {
			info := s.getInfoForNode(lhs)
			if info == nil {
				continue
			}
			obj := info.Defs[ident]
			if obj != nil {
				// The variable `obj` now holds our tracked value.
				s.VarValues[obj] = item.Value
				// Now find all usages of this variable and add them to the worklist.
				s.findAndQueueUsages(obj, item.Value)
			}
		}
	}
}

// traceMethodCall handles cases where a method is called on a tracked value.
func (s *State) traceMethodCall(item WorklistItem, call *ast.CallExpr) {
	// The item.Node is a SelectorExpr, e.g., `r.Get`
	selExpr := item.Node.(*ast.SelectorExpr)
	methodName := selExpr.Sel.Name
	routerDef := item.Value.RouterDef

	// Case 1: Is this method an endpoint registration (a "sink")?
	for _, endpointMethod := range routerDef.EndpointMethods {
		if methodName == endpointMethod {
			fmt.Printf("  [Sink] Found route call: %s\n", methodName)
			s.buildRouteFromCall(item, call) // Call the Phase 4 builder
			return                           // Stop tracing this branch
		}
	}

	// Case 2: Is this a chaining method (group or middleware)?
	isChain := false
	for _, groupMethod := range routerDef.GroupMethods {
		if methodName == groupMethod {
			isChain = true
			break
		}
	}
	if !isChain {
		for _, mwMethod := range routerDef.MiddlewareWrapperMethods {
			if methodName == mwMethod {
				isChain = true
				break
			}
		}
	}

	if isChain {
		fmt.Printf("  [Chain] Found chaining call: %s\n", methodName)
		// // This call transforms the router. The result of the call is a *new* value to track.
		// var pathPrefix string
		// // If it's a group method, try to resolve the path prefix from the first argument.
		// if len(call.Args) > 0 {
		// 	// This assumes group methods have the path as the first argument.
		// 	path, ok := s.resolveStringValue(call.Args[0])
		// 	if ok {
		// 		pathPrefix = path
		// 	}
		// }

		// This call transforms the router. The result is a new value to track.
		newParentNode := item.Value.Node                    // The new node is a child of the current one
		pathPrefix, _ := s.resolveStringValue(call.Args[0]) // Simplified

		// Create a new graph node for the group
		newNode := &model.RouteNode{PathPrefix: pathPrefix, Parent: newParentNode}
		newParentNode.Children = append(newParentNode.Children, newNode)

		newVal := &TrackedValue{
			Source:     call,
			RouterDef:  routerDef,
			Parent:     item.Value,
			PathPrefix: pathPrefix,
			Node:       newNode, // Associate the new graph node
		}

		// The call expression itself now represents the new trackable value.
		s.ExprResults[call] = newVal
		// Add the call expression to the worklist to trace what happens to the new group/router.
		s.Worklist = append(s.Worklist, WorklistItem{
			Node:  call,
			Value: newVal,
		})
	}
}

// findAndQueueUsages finds all references to a given variable object and adds them to the worklist.
func (s *State) findAndQueueUsages(obj types.Object, value *TrackedValue) {
	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			info := s.fileTypeInfo[file]
			ast.Inspect(file, func(n ast.Node) bool {
				if ident, ok := n.(*ast.Ident); ok {
					if info.Uses[ident] == obj {
						// Found a usage of our variable. Add it to the worklist to be traced.
						s.Worklist = append(s.Worklist, WorklistItem{
							Node:  ident,
							Value: value,
						})
					}
				}
				return true
			})
		}
	}
}

// --- UTILITY FUNCTIONS ---

// findPathToNode finds the chain of parent nodes from the file root to the target node.
func (s *State) findPathToNode(target ast.Node) ([]ast.Node, bool) {
	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			// Check if the node is within this file's position range.
			if file.Pos() <= target.Pos() && target.End() <= file.End() {
				path, exact := astutil.PathEnclosingInterval(file, target.Pos(), target.End())
				if exact {
					return path, true
				}
			}
		}
	}
	return nil, false
}

// getInfoForNode is a helper to find the correct types.Info for any given node.
func (s *State) getInfoForNode(node ast.Node) *types.Info {
	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			if file.Pos() <= node.Pos() && node.End() <= file.End() {
				return s.fileTypeInfo[file]
			}
		}
	}
	return nil
}

// ... (the rest of flow.go: findSources, isResolvedRouterType, etc.)

// findSources scans the entire AST for calls to known router initializers
// and adds them as the initial items to the worklist.
func (s *State) findSources() {
	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			info := s.fileTypeInfo[file]
			if info == nil {
				continue
			}

			ast.Inspect(file, func(n ast.Node) bool {
				callExpr, ok := n.(*ast.CallExpr)
				if !ok {
					return true // Continue traversal
				}

				// Is this call producing a value of a type we care about?
				obj := info.Uses[getIdentifier(callExpr.Fun)]
				if fn, isFunc := obj.(*types.Func); isFunc {
					sig := fn.Signature()
					// Check if the function returns a single value
					if sig.Results().Len() == 1 {
						retType := sig.Results().At(0).Type()
						if resolvedType := s.isResolvedRouterType(retType); resolvedType != nil {
							// SOURCE FOUND! This is a call to a router initializer.
							// This is the root of a tracking chain.
							node := &model.RouteNode{GoVar: nil, Parent: s.RouteGraph} // Create a graph node
							s.RouteGraph.Children = append(s.RouteGraph.Children, node)

							trackedVal := &TrackedValue{
								Source:    callExpr,
								RouterDef: resolvedType.Definition,
								Node:      node, // Associate the graph node
							}
							// Link the expression result to this new value.
							s.ExprResults[callExpr] = trackedVal

							// Add this call expression to the worklist to be analyzed.
							s.Worklist = append(s.Worklist, WorklistItem{
								Node:  callExpr,
								Value: trackedVal,
							})
						}
					}
				}
				return true
			})
		}
	}
}

// isResolvedRouterType checks if a given type 't' matches one of the router types
// we resolved in Phase 1. It returns the corresponding ResolvedType if it's a match.
func (s *State) isResolvedRouterType(t types.Type) *ResolvedType {
	var named *types.Named
	if ptr, isPtr := t.(*types.Pointer); isPtr {
		if n, isNamed := ptr.Elem().(*types.Named); isNamed {
			named = n
		}
	} else if n, isNamed := t.(*types.Named); isNamed {
		named = n
	}

	if named == nil {
		return nil
	}

	// Check against all the types we resolved from the config.
	for _, resolvedType := range s.ResolvedRouterTypes {
		if named.Obj() == resolvedType.Object.Obj() {
			return resolvedType
		}
	}
	return nil
}

// getIdentifier is a simple helper to get the *ast.Ident from a call's Fun expression.
func getIdentifier(expr ast.Expr) *ast.Ident {
	switch e := expr.(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return e.Sel
	}
	return nil
}

// traceFunctionArgument handles the case where a tracked value is passed as an
// argument to a function call. It traces the flow into that function.
func (s *State) traceFunctionArgument(item WorklistItem, call *ast.CallExpr, argIndex int) {
	// 1. Find the function being called.
	calleeObj := s.getObjectForExpr(call.Fun)
	if calleeObj == nil {
		return
	}

	// 2. Look up the function's declaration in our universe.
	funcDecl, ok := s.Universe.Functions[calleeObj]
	if !ok || funcDecl.Type == nil || funcDecl.Type.Params == nil || funcDecl.Body == nil {
		return // We can't analyze this function.
	}

	// 3. Find the parameter object corresponding to our argument.
	if len(funcDecl.Type.Params.List) <= argIndex {
		return
	}
	paramField := funcDecl.Type.Params.List[argIndex]
	if len(paramField.Names) == 0 {
		return
	}
	paramIdent := paramField.Names[0]
	info := s.getInfoForNode(paramIdent)
	paramObj := info.Defs[paramIdent]
	if paramObj == nil {
		return
	}

	// 4. The parameter inside the function now holds our tracked value.
	fmt.Printf("  [Flow] Tracing value into function '%s' via parameter '%s'\n", calleeObj.Name(), paramObj.Name())
	s.VarValues[paramObj] = item.Value

	// 5. Find all usages of THAT PARAMETER within the function's body and queue them.
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			// Check if this identifier is a usage of our parameter
			if usageInfo := s.getInfoForNode(ident); usageInfo != nil && usageInfo.Uses[ident] == paramObj {
				s.Worklist = append(s.Worklist, WorklistItem{
					Node:  ident,
					Value: item.Value, // Pass the same value down
				})
			}
		}
		return true
	})
}
