package analyzer

import (
	"go/ast"

	"github.com/Zachacious/go-respec/internal/meta"
	"github.com/Zachacious/go-respec/internal/model"
)

// FindGroupMetadata scans the project for `respec.Meta(r)` calls and builds a
// map associating the router variable `r` with its chained metadata.
func (s *State) FindGroupMetadata() {
	// Use the type from the model package.
	s.GroupMetadata = make(model.GroupMetadataMap)

	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				endCall, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				metaCall, builder := s.parseMetaChain(endCall)
				if metaCall == nil {
					return true
				}

				if len(metaCall.Args) == 1 {
					routerVarIdent, ok := metaCall.Args[0].(*ast.Ident)
					if !ok {
						return true
					}

					if info := s.getInfoForNode(routerVarIdent); info != nil {
						if routerVarObj := info.Uses[routerVarIdent]; routerVarObj != nil {
							s.GroupMetadata[routerVarObj] = builder
							return false
						}
					}
				}
				return true
			})
		}
	}
}

// parseMetaChain walks a chain of calls like `respec.Meta(...).Tag(...).Security(...)` backwards.
func (s *State) parseMetaChain(endCall *ast.CallExpr) (*ast.CallExpr, *meta.GroupMetadata) {
	group := &meta.GroupMetadata{}
	currentCall := endCall

	for {
		selExpr, ok := currentCall.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}

		var argValues []string
		for _, arg := range currentCall.Args {
			if val, ok := s.resolveStringValue(arg); ok {
				argValues = append(argValues, val)
			}
		}

		switch selExpr.Sel.Name {
		case "Tag":
			group.Tags = append(group.Tags, argValues...)
		case "Security":
			// Every argument counts. Reading only the first silently dropped
			// the rest of a multi-scheme requirement.
			group.Security = append(group.Security, argValues...)
		case "Public":
			group.NoSecurity = true
		case "Deprecated":
			group.Deprecated = true
		case "Deprecate":
			if len(currentCall.Args) > 0 {
				if val, ok := getBoolValue(currentCall.Args[0]); ok {
					group.Deprecated = val
				}
			}
		}

		prevCall, ok := selExpr.X.(*ast.CallExpr)
		if !ok {
			break
		}
		currentCall = prevCall
	}

	obj := s.getObjectForExpr(currentCall.Fun)
	if obj == nil || getFuncPath(obj) != "github.com/Zachacious/go-respec/respec.Meta" {
		return nil, nil
	}

	return currentCall, group
}
