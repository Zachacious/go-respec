package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/Zachacious/go-respec/internal/meta"
)

// SpecSite is one `respec.Handler(...)....Unwrap()` chain found in the source.
//
// Metadata is keyed by the site rather than by the handler's types.Object,
// because a handler object is not a unique key: an anonymous function has none,
// and a handler mounted on two routes would otherwise share (and overwrite) a
// single metadata entry.
type SpecSite struct {
	// Metadata parsed from the builder chain.
	Metadata *meta.HandlerMetadata
	// HandlerExpr is the argument given to respec.Handler.
	HandlerExpr ast.Expr
	// HandlerObj is the handler's object, when it is a named function. Nil for
	// function literals.
	HandlerObj types.Object
	// HandlerBody is the code to scan for schemas and parameters, from either a
	// declaration or a literal.
	HandlerBody *ast.BlockStmt
	// Name is a human-readable handler name used for summaries and operation IDs.
	Name string
	// Pos anchors diagnostics to the builder chain.
	Pos token.Pos
	// Linked records whether any route resolved to this site. An unlinked site
	// means the user wrote metadata that never reached the spec.
	Linked bool
}

// resolvedHandler is the outcome of resolving a route's handler argument.
type resolvedHandler struct {
	Site *SpecSite
	Obj  types.Object
	Body *ast.BlockStmt
	Name string
}

// FindAndParseRouteMetadata locates every builder chain and indexes it by the
// ways a route can refer to it: inline, through a variable, or through a
// factory function.
func (s *State) FindAndParseRouteMetadata() {
	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Unwrap" {
					return true
				}

				metadata, handlerExpr := s.parseHandlerChain(sel.X)
				if handlerExpr == nil {
					return true
				}

				site := s.newSpecSite(metadata, handlerExpr, call)
				s.SpecSites = append(s.SpecSites, site)
				s.siteByUnwrap[call] = site
				s.indexSiteByContext(call, site)
				return false
			})
		}
	}
}

// newSpecSite resolves the handler expression to an object and a body.
func (s *State) newSpecSite(metadata *meta.HandlerMetadata, handlerExpr ast.Expr, unwrap *ast.CallExpr) *SpecSite {
	site := &SpecSite{
		Metadata:    metadata,
		HandlerExpr: handlerExpr,
		Pos:         unwrap.Pos(),
	}

	if lit, ok := handlerExpr.(*ast.FuncLit); ok {
		// An inline handler has no object, but its body is right here and is
		// just as analyzable as a declared function's.
		site.HandlerBody = lit.Body
		site.Name = s.enclosingFuncName(unwrap)
		return site
	}

	if obj := s.getObjectForExpr(handlerExpr); obj != nil {
		site.HandlerObj = obj
		site.Name = obj.Name()
		if decl, ok := s.Universe.Functions[obj]; ok {
			site.HandlerBody = decl.Body
		}
	}
	return site
}

// indexSiteByContext records how a builder chain can be reached from a route:
// as the value of a variable, or as the return value of a factory function.
func (s *State) indexSiteByContext(unwrap *ast.CallExpr, site *SpecSite) {
	path, found := s.findPathToNode(unwrap)
	if !found {
		return
	}

	for _, n := range path[1:] {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			// h := respec.Handler(x)....Unwrap()
			for i, rhs := range stmt.Rhs {
				if rhs != ast.Expr(unwrap) || i >= len(stmt.Lhs) {
					continue
				}
				if ident, ok := stmt.Lhs[i].(*ast.Ident); ok {
					if info := s.getInfoForNode(ident); info != nil {
						obj := info.Defs[ident]
						if obj == nil {
							obj = info.Uses[ident]
						}
						if obj != nil {
							s.siteByVar[obj] = site
						}
					}
				}
			}
			return

		case *ast.ValueSpec:
			for i, v := range stmt.Values {
				if v != ast.Expr(unwrap) || i >= len(stmt.Names) {
					continue
				}
				if info := s.getInfoForNode(stmt.Names[i]); info != nil {
					if obj := info.Defs[stmt.Names[i]]; obj != nil {
						s.siteByVar[obj] = site
					}
				}
			}
			return

		case *ast.FuncDecl:
			// return respec.Handler(x)....Unwrap() inside a factory function.
			if info := s.getInfoForNode(stmt.Name); info != nil {
				if obj := info.Defs[stmt.Name]; obj != nil {
					s.siteByFunc[obj] = site
					if site.Name == "" {
						site.Name = obj.Name()
					}
				}
			}
			return
		}
	}
}

// enclosingFuncName names the function a node sits inside, used to label
// handlers written as anonymous functions.
//
// It returns "" rather than a placeholder when there is no useful name. A
// summary of "handler", or of "main" for a literal declared in main, is worse
// than no summary at all: it looks like a real name and tells the reader nothing.
func (s *State) enclosingFuncName(n ast.Node) string {
	path, found := s.findPathToNode(n)
	if !found {
		return ""
	}
	for _, p := range path {
		fd, ok := p.(*ast.FuncDecl)
		if !ok || fd.Name == nil {
			continue
		}
		// A factory such as healthSpec() names its handler usefully; main and
		// init do not.
		if fd.Name.Name == "main" || fd.Name.Name == "init" {
			return ""
		}
		return fd.Name.Name
	}
	return ""
}

// resolveHandler maps a route's handler argument onto a builder chain and a
// body to analyze.
//
// The cases below are the ways a handler is written in practice. Only the first
// was previously understood; the rest silently produced no route at all.
func (s *State) resolveHandler(expr ast.Expr) resolvedHandler {
	switch e := expr.(type) {
	case *ast.CallExpr:
		// Inline: respec.Handler(h.Create)....Unwrap()
		if site, ok := s.siteByUnwrap[e]; ok {
			return handlerFromSite(site)
		}
		// Factory: h.CreateAddressSpec()
		if funcObj := s.getObjectForExpr(e.Fun); funcObj != nil {
			if site, ok := s.siteByFunc[funcObj]; ok {
				// Worth surfacing under --verbose: the handler documented here
				// is not the value named at the call site, so anyone checking
				// the spec against the route can see how the two were connected.
				s.infof(e.Pos(), "resolved %s() to handler %s through its return value", funcObj.Name(), site.Name)
				return handlerFromSite(site)
			}
			// A factory with no builder chain still yields a real handler if we
			// can see what it returns.
			if body := s.returnedHandlerBody(funcObj); body != nil {
				return resolvedHandler{Obj: funcObj, Body: body, Name: funcObj.Name()}
			}
		}
		// A conversion such as http.HandlerFunc(myHandler).
		if info := s.getInfoForNode(e.Fun); info != nil {
			if tv, ok := info.Types[e.Fun]; ok && tv.IsType() && len(e.Args) == 1 {
				return s.resolveHandler(e.Args[0])
			}
		}

	case *ast.Ident:
		obj := s.getObjectForExpr(e)
		if obj == nil {
			break
		}
		// A variable holding an unwrapped builder result.
		if site, ok := s.siteByVar[obj]; ok {
			return handlerFromSite(site)
		}
		if decl, ok := s.Universe.Functions[obj]; ok {
			return resolvedHandler{Obj: obj, Body: decl.Body, Name: obj.Name()}
		}
		return resolvedHandler{Obj: obj, Name: obj.Name()}

	case *ast.SelectorExpr:
		if obj := s.getObjectForExpr(e); obj != nil {
			if site, ok := s.siteByVar[obj]; ok {
				return handlerFromSite(site)
			}
			var body *ast.BlockStmt
			if decl, ok := s.Universe.Functions[obj]; ok {
				body = decl.Body
			}
			return resolvedHandler{Obj: obj, Body: body, Name: obj.Name()}
		}

	case *ast.FuncLit:
		// A bare anonymous handler written straight into the route call. It is
		// deliberately left unnamed: the function it sits inside is the router
		// setup, so borrowing that name labels every inline handler after the
		// constructor that registered it. No summary is better than a wrong one.
		return resolvedHandler{Body: e.Body}
	}

	return resolvedHandler{}
}

// returnedHandlerBody finds the body a factory function hands back, so a
// handler built without a respec chain can still be analyzed.
func (s *State) returnedHandlerBody(funcObj types.Object) *ast.BlockStmt {
	decl, ok := s.Universe.Functions[funcObj]
	if !ok || decl.Body == nil {
		return nil
	}
	var body *ast.BlockStmt
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 || body != nil {
			return true
		}
		if lit, ok := ret.Results[0].(*ast.FuncLit); ok {
			body = lit.Body
			return false
		}
		return true
	})
	return body
}

func handlerFromSite(site *SpecSite) resolvedHandler {
	site.Linked = true
	return resolvedHandler{
		Site: site,
		Obj:  site.HandlerObj,
		Body: site.HandlerBody,
		Name: site.Name,
	}
}

// reportUnlinkedSites warns about builder chains that never reached a route.
// This is the diagnostic that makes a wrapped handler self-explanatory instead
// of just missing.
func (s *State) reportUnlinkedSites() {
	for _, site := range s.SpecSites {
		if site.Linked {
			continue
		}
		name := site.Name
		if name == "" {
			name = "handler"
		}
		s.reportf(site.Pos,
			"respec.Handler(%s) metadata was never attached to a route; check that the value returned by .Unwrap() is registered on a router respec is tracking",
			name)
	}
}
