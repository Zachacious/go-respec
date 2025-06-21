package analyzer

import (
	"go/ast"

	"github.com/Zachacious/go-respec/respec"
)

func (a *Analyzer) parseBuilderChain(call *ast.CallExpr) *respec.Builder {
	b := respec.NewBuilder()
	currentCall := call

	for {
		selExpr, ok := currentCall.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}
		methodName := selExpr.Sel.Name

		if len(currentCall.Args) > 0 {
			switch methodName {
			case "Summary":
				if summary, ok := a.getStringFromExpr(currentCall.Args[0]); ok {
					b.Summary(summary)
				}
			case "Description":
				if desc, ok := a.getStringFromExpr(currentCall.Args[0]); ok {
					b.Description(desc)
				}
			}
		}

		if nextCall, ok := selExpr.X.(*ast.CallExpr); ok {
			currentCall = nextCall
		} else {
			break
		}
	}
	return b
}

// func (a *Analyzer) getStringFromExpr(expr ast.Expr) (string, bool) {
// 	lit, ok := expr.(*ast.BasicLit)
// 	if !ok || lit.Kind != token.STRING {
// 		return "", false
// 	}
// 	// FIX: Correctly handle the (string, error) tuple from Unquote.
// 	s, err := strconv.Unquote(lit.Value)
// 	if err != nil {
// 		return "", false
// 	}
// 	return s, true
// }
