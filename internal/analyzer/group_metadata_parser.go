package analyzer

import (
	"go/ast"
	"strconv"

	"github.com/Zachacious/go-respec/internal/model" // Import the model package
	"github.com/Zachacious/go-respec/respec"
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
func (s *State) parseMetaChain(endCall *ast.CallExpr) (*ast.CallExpr, *respec.GroupBuilder) {
	builder := respec.NewGroupBuilder()
	currentCall := endCall

	for {
		selExpr, ok := currentCall.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}

		methodName := selExpr.Sel.Name
		var argValues []string
		for _, arg := range currentCall.Args {
			if lit, ok := arg.(*ast.BasicLit); ok {
				if val, err := strconv.Unquote(lit.Value); err == nil {
					argValues = append(argValues, val)
				}
			}
		}

		switch methodName {
		case "Tag":
			builder.Tag(argValues...)
		case "Security":
			if len(argValues) > 0 {
				builder.Security(argValues[0])
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

	return currentCall, builder
}
