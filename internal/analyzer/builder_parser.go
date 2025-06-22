package analyzer

import (
	"go/ast"
	"strconv"

	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
)

// GroupMetadata holds the parsed metadata from a respec.Group call.
type GroupMetadata struct {
	Tags     []string
	Security []string
}

// FindAndApplyGroupMetadata finds all `respec.Group` calls and applies their
// metadata to all operations defined within their scope.
func (s *State) FindAndApplyGroupMetadata() {
	for _, pkg := range s.pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				obj := s.getObjectForExpr(call.Fun)
				// FIX: Look for the new function name `respec.Group`.
				if obj == nil || getFuncPath(obj) != "github.com/Zachacious/go-respec/respec.Group" {
					return true
				}

				metadata := parseGroupChain(call)

				if len(call.Args) == 1 {
					if funcLit, ok := call.Args[0].(*ast.FuncLit); ok && funcLit.Body != nil {
						s.applyMetadataToBlock(funcLit.Body, metadata)
					}
				}
				return true
			})
		}
	}
}

// parseGroupChain walks backwards from the end of a chain like `...Group(...).Tag(...)`
func parseGroupChain(startCall *ast.CallExpr) GroupMetadata {
	metadata := GroupMetadata{}
	currentCall := startCall

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
			metadata.Tags = append(metadata.Tags, argValues...)
		case "Security":
			if len(argValues) > 0 {
				metadata.Security = append(metadata.Security, argValues[0])
			}
		}

		prevCall, ok := selExpr.X.(*ast.CallExpr)
		if !ok {
			break
		}
		currentCall = prevCall
	}
	return metadata
}

// applyMetadataToBlock finds all operations within a given AST block and adds the metadata.
func (s *State) applyMetadataToBlock(block *ast.BlockStmt, metadata GroupMetadata) {
	s.traverseAndApply(s.RouteGraph, block, metadata)
}

func (s *State) traverseAndApply(node *model.RouteNode, block *ast.BlockStmt, metadata GroupMetadata) {
	for _, op := range node.Operations {
		if op.GoHandler != nil && block.Pos() <= op.GoHandler.Pos() && op.GoHandler.Pos() < block.End() {
			if op.Spec != nil {
				op.Spec.Tags = append(metadata.Tags, op.Spec.Tags...)

				if op.Spec.Security == nil && len(metadata.Security) > 0 {
					req := openapi3.SecurityRequirement{}
					for _, schemeName := range metadata.Security {
						req[schemeName] = []string{}
					}
					op.Spec.Security = &openapi3.SecurityRequirements{req}
				}
			}
		}
	}
	for _, child := range node.Children {
		s.traverseAndApply(child, block, metadata)
	}
}
