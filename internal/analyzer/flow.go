package analyzer

import (
	"go/ast"
	"go/types"
	"slices"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
	"golang.org/x/tools/go/ast/astutil"
)

// performDataFlowAnalysis traces every router value through the program.
//
// A router reaches the call that registers a route by several routes of its
// own: method chaining, a closure parameter, a plain assignment, an argument to
// a helper function, or a return value that is later mounted. Missing any one
// of them does not degrade the spec, it silently deletes routes, so each is
// handled explicitly below.
func (s *State) performDataFlowAnalysis() {
	for _, v := range s.findInitialRouterVars() {
		s.findAndProcessUsages(v)
	}

	// Mounting can re-parent a sub-router that was already analyzed, so paths
	// are only assembled once the graph has stopped moving.
	s.assignFullPaths(s.RouteGraph)
}

// findInitialRouterVars finds variables initialised directly from a constructor
// returning a known router type, e.g. `r := chi.NewRouter()`.
func (s *State) findInitialRouterVars() []types.Object {
	var initialVars []types.Object

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
				sig, ok := info.TypeOf(callExpr.Fun).(*types.Signature)
				if !ok || sig.Results().Len() != 1 {
					return true
				}
				resolvedType := s.isResolvedRouterType(sig.Results().At(0).Type())
				if resolvedType == nil {
					return true
				}
				ident, ok := assign.Lhs[0].(*ast.Ident)
				if !ok {
					return true
				}
				obj, _ := info.Defs[ident].(*types.Var)
				if obj == nil {
					return true
				}

				node := &model.RouteNode{GoVar: obj, Parent: s.RouteGraph}
				s.RouteGraph.Children = append(s.RouteGraph.Children, node)
				s.VarValues[obj] = &TrackedValue{
					Source:    callExpr,
					RouterDef: resolvedType.Definition,
					Node:      node,
				}
				initialVars = append(initialVars, obj)
				return true
			})
		}
	}
	return initialVars
}

// bind associates an object with a tracked router and analyzes its uses. It is
// the single entry point for extending the reach of a router value, and it
// refuses to revisit an object so that mutually recursive helpers terminate.
func (s *State) bind(obj types.Object, val *TrackedValue) {
	if obj == nil || val == nil {
		return
	}
	if existing, ok := s.VarValues[obj]; ok && existing == val {
		return
	}
	if s.boundObjects[obj] {
		return
	}
	s.boundObjects[obj] = true
	s.VarValues[obj] = val
	s.findAndProcessUsages(obj)
}

// findAndProcessUsages walks every reference to a tracked router variable.
func (s *State) findAndProcessUsages(v types.Object) {
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

				switch parent := path[1].(type) {
				case *ast.SelectorExpr:
					// r.Get(...), r.Route(...), r.Mount(...)
					if len(path) > 2 {
						if callExpr, ok := path[2].(*ast.CallExpr); ok && callExpr.Fun == parent {
							s.processMethodCall(initialValue, callExpr, file)
						}
					}
				case *ast.CallExpr:
					// The router is being passed to a function: RegisterRoutes(r).
					// Without this, every route registered inside that function
					// is invisible.
					if parent.Fun != ident {
						s.propagateThroughCallArgument(initialValue, parent, ident)
					}
				}
				return true
			})
		}
	}
}

// propagateThroughCallArgument binds a callee's parameter to the router that
// was passed in, so routes registered inside the callee land on the caller's
// routing scope.
func (s *State) propagateThroughCallArgument(val *TrackedValue, call *ast.CallExpr, arg *ast.Ident) {
	argIndex := slices.IndexFunc(call.Args, func(e ast.Expr) bool { return e == arg })
	if argIndex < 0 {
		return
	}

	funcObj := s.getObjectForExpr(call.Fun)
	if funcObj == nil {
		return
	}
	funcDecl, ok := s.Universe.Functions[funcObj]
	if !ok || funcDecl.Type.Params == nil {
		return
	}

	// Flatten the parameter list: `func(a, b chi.Router)` is one field, two names.
	var params []*ast.Ident
	for _, field := range funcDecl.Type.Params.List {
		if len(field.Names) == 0 {
			params = append(params, nil) // unnamed parameter still occupies a slot
			continue
		}
		params = append(params, field.Names...)
	}
	if argIndex >= len(params) || params[argIndex] == nil {
		return
	}

	info := s.getInfoForNode(params[argIndex])
	if info == nil {
		return
	}
	s.bind(info.Defs[params[argIndex]], val)
}

// routerValueForExpr resolves an expression to a router that is already being
// tracked. It understands a variable holding a router and a call to a function
// that builds and returns one, which together cover how sub-routers are written.
func (s *State) routerValueForExpr(expr ast.Expr) *TrackedValue {
	switch e := expr.(type) {
	case *ast.Ident:
		if obj := s.getObjectForExpr(e); obj != nil {
			return s.VarValues[obj]
		}
	case *ast.CallExpr:
		funcObj := s.getObjectForExpr(e.Fun)
		if funcObj == nil {
			return nil
		}
		funcDecl, ok := s.Universe.Functions[funcObj]
		if !ok || funcDecl.Body == nil {
			return nil
		}
		// Find what the function returns and resolve that instead.
		var found *TrackedValue
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 || found != nil {
				return true
			}
			found = s.routerValueForExpr(ret.Results[0])
			return found == nil
		})
		return found
	}
	return nil
}

// processMethodCall dispatches a method call made on a tracked router.
func (s *State) processMethodCall(currentValue *TrackedValue, call *ast.CallExpr, file *ast.File) {
	selExpr, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	if s.processed[processKey{node: call, val: currentValue}] {
		return
	}
	s.processed[processKey{node: call, val: currentValue}] = true

	methodName := selExpr.Sel.Name
	routerDef := currentValue.RouterDef

	if pattern, isEndpoint := endpointPatternFor(routerDef, methodName); isEndpoint {
		s.buildRouteFromCall(currentValue, call, pattern, methodName)
		return
	}

	if slices.Contains(routerDef.MountMethods, methodName) {
		s.processMount(currentValue, call)
		return
	}

	isGroupMethod := slices.Contains(routerDef.GroupMethods, methodName)
	isMiddlewareMethod := slices.Contains(routerDef.MiddlewareWrapperMethods, methodName)
	if !isGroupMethod && !isMiddlewareMethod {
		return
	}

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

	if isMiddlewareMethod {
		for _, arg := range call.Args {
			if middlewareObj := s.getObjectForExpr(arg); middlewareObj != nil {
				newNode.InferredSecurity = append(newNode.InferredSecurity, s.analyzeMiddleware(middlewareObj)...)
			}
		}
	}

	// Chained: r.With(mw).Get(...) or r.Group("/v1").GET(...)
	if path, found := s.findPathToNode(call); found && len(path) > 2 {
		if parentSel, ok := path[1].(*ast.SelectorExpr); ok && parentSel.X == call {
			if parentCall, ok := path[2].(*ast.CallExpr); ok && parentCall.Fun == parentSel {
				s.processMethodCall(newVal, parentCall, file)
			}
		}
	}

	// Closure form: r.Route("/users", func(r chi.Router) { ... })
	for _, arg := range call.Args {
		funcLit, ok := arg.(*ast.FuncLit)
		if !ok || funcLit.Type.Params == nil || len(funcLit.Type.Params.List) == 0 {
			continue
		}
		names := funcLit.Type.Params.List[0].Names
		if len(names) == 0 {
			continue
		}
		info := s.getInfoForNode(names[0])
		if info == nil {
			continue
		}
		if paramObj, ok := info.Defs[names[0]].(*types.Var); ok {
			newNode.GoVar = paramObj
			s.bind(paramObj, newVal)
		}
	}

	// Assigned form: v1 := r.Group("/v1"). This is the dominant gin and echo
	// idiom, and without it every route on the group disappears.
	if path, found := s.findPathToNode(call); found && len(path) > 1 {
		s.bindAssignmentTarget(path, call, newVal, newNode)
	}
}

// bindAssignmentTarget handles `v := r.Group(...)` and `var v = r.Group(...)`.
func (s *State) bindAssignmentTarget(path []ast.Node, call *ast.CallExpr, newVal *TrackedValue, newNode *model.RouteNode) {
	for _, n := range path[1:] {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			for i, rhs := range stmt.Rhs {
				if rhs != call || i >= len(stmt.Lhs) {
					continue
				}
				ident, ok := stmt.Lhs[i].(*ast.Ident)
				if !ok {
					continue
				}
				info := s.getInfoForNode(ident)
				if info == nil {
					continue
				}
				obj := info.Defs[ident]
				if obj == nil {
					obj = info.Uses[ident]
				}
				if obj != nil {
					if newNode.GoVar == nil {
						newNode.GoVar = obj
					}
					s.bind(obj, newVal)
				}
			}
			return
		case *ast.ValueSpec:
			for i, rhs := range stmt.Values {
				if rhs != call || i >= len(stmt.Names) {
					continue
				}
				info := s.getInfoForNode(stmt.Names[i])
				if info == nil {
					continue
				}
				if obj := info.Defs[stmt.Names[i]]; obj != nil {
					if newNode.GoVar == nil {
						newNode.GoVar = obj
					}
					s.bind(obj, newVal)
				}
			}
			return
		case *ast.ExprStmt, *ast.FuncDecl, *ast.BlockStmt:
			return
		}
	}
}

// processMount attaches an existing sub-router beneath a path prefix.
//
// The sub-router was discovered as a top-level router in its own right, so its
// node is moved out of the graph root and re-parented here. Paths are assembled
// after all analysis completes, which is what makes the move safe.
func (s *State) processMount(currentValue *TrackedValue, call *ast.CallExpr) {
	if len(call.Args) < 2 {
		return
	}
	prefix, ok := s.resolveStringValue(call.Args[0])
	if !ok {
		return
	}

	target := s.routerValueForExpr(call.Args[1])
	if target == nil || target.Node == nil {
		// Not a router we know about - most likely a plain http.Handler.
		s.reportf(call.Pos(), "mount at %q: could not resolve the mounted value to a known router; its routes will be missing", prefix)
		return
	}
	if target.Node == currentValue.Node {
		return
	}

	mountNode := &model.RouteNode{PathPrefix: prefix, Parent: currentValue.Node}
	currentValue.Node.Children = append(currentValue.Node.Children, mountNode)

	// Detach from the previous parent before re-attaching.
	if old := target.Node.Parent; old != nil {
		old.Children = slices.DeleteFunc(old.Children, func(c *model.RouteNode) bool {
			return c == target.Node
		})
	}
	target.Node.Parent = mountNode
	mountNode.Children = append(mountNode.Children, target.Node)
	target.Parent = &TrackedValue{
		RouterDef:  currentValue.RouterDef,
		PathPrefix: prefix,
		Parent:     currentValue,
		Node:       mountNode,
	}
}

// assignFullPaths walks the finished graph and resolves each operation's path
// against its ancestors.
func (s *State) assignFullPaths(node *model.RouteNode) {
	for _, op := range node.Operations {
		op.FullPath = buildFullPath(node, op.RoutePath)
	}
	for _, child := range node.Children {
		s.assignFullPaths(child)
	}
}

// buildFullPath joins every ancestor prefix with the route's own path.
func buildFullPath(node *model.RouteNode, routePath string) string {
	var prefixes []string
	for n := node; n != nil; n = n.Parent {
		// The synthetic graph root carries "/" which would double the separator.
		if n.Parent != nil && n.PathPrefix != "" {
			prefixes = append(prefixes, n.PathPrefix)
		}
	}

	var b []byte
	for i := len(prefixes) - 1; i >= 0; i-- {
		b = append(b, trimTrailingSlash(prefixes[i])...)
	}

	// A route path of "/" is the group's own index, so it contributes nothing
	// beyond the prefix: Route("/users") + Get("/") is /users, not /users/.
	// Any other path is used as written, because a trailing slash can be
	// meaningful - net/http treats "/static/" as a subtree match, and it is a
	// different OpenAPI path from "/static".
	if routePath != "" && routePath != "/" {
		if routePath[0] != '/' {
			b = append(b, '/')
		}
		b = append(b, routePath...)
	}

	if len(b) == 0 {
		return "/"
	}
	return string(b)
}

func trimTrailingSlash(s string) string {
	if len(s) > 1 && s[len(s)-1] == '/' {
		return s[:len(s)-1]
	}
	if s == "/" {
		return ""
	}
	return s
}

// endpointPatternFor reports whether a method registers a route, and how its
// arguments are laid out.
func endpointPatternFor(def *config.RouterDefinition, methodName string) (config.EndpointPattern, bool) {
	for _, p := range def.EndpointPatterns {
		if slices.Contains(p.Methods, methodName) {
			return p, true
		}
	}
	if slices.Contains(def.EndpointMethods, methodName) {
		// Default shape: the verb is the method name itself.
		return config.EndpointPattern{PathArg: 0, HandlerArg: 1}, true
	}
	return config.EndpointPattern{}, false
}
