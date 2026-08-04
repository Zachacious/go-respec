package analyzer

import (
	"go/ast"
	"go/token"
	"strconv"

	"github.com/Zachacious/go-respec/internal/meta"
)

// parseHandlerChain walks a `respec.Handler(x).A().B().Unwrap()` chain backwards,
// collecting metadata until it reaches the Handler call.
//
// It returns the metadata and the expression handed to Handler, or nil if the
// chain does not terminate in a respec.Handler call.
func (s *State) parseHandlerChain(expr ast.Expr) (*meta.HandlerMetadata, ast.Expr) {
	metadata := &meta.HandlerMetadata{}
	currentExpr := expr

	// Options are applied to whichever response or parameter is being built, so
	// the chain is read in source order rather than the reverse order it is
	// encountered in.
	var calls []*ast.CallExpr
	for {
		call, isCall := currentExpr.(*ast.CallExpr)
		if !isCall {
			break
		}
		sel, isSel := call.Fun.(*ast.SelectorExpr)
		if !isSel {
			break
		}
		calls = append(calls, call)
		if sel.Sel.Name == "Handler" {
			break
		}
		currentExpr = sel.X
	}

	var handlerExpr ast.Expr
	for i := len(calls) - 1; i >= 0; i-- {
		call := calls[i]
		sel := call.Fun.(*ast.SelectorExpr)

		switch sel.Sel.Name {
		case "Handler":
			if len(call.Args) != 1 {
				return nil, nil
			}
			handlerExpr = call.Args[0]

		case "Summary":
			if len(call.Args) > 0 {
				if str, ok := s.resolveStringValue(call.Args[0]); ok {
					metadata.Summary = str
				}
			}
		case "Description":
			if len(call.Args) > 0 {
				if str, ok := s.resolveStringValue(call.Args[0]); ok {
					metadata.Description = str
				}
			}
		case "Tag":
			for _, arg := range call.Args {
				if str, ok := s.resolveStringValue(arg); ok {
					metadata.Tags = append(metadata.Tags, str)
				}
			}
		case "Security":
			for _, arg := range call.Args {
				if str, ok := s.resolveStringValue(arg); ok {
					metadata.Security = append(metadata.Security, str)
				}
			}
		case "Public":
			metadata.NoSecurity = true
		case "Hide":
			metadata.Hidden = true
		case "RequestBody":
			if len(call.Args) > 0 {
				metadata.RequestBodyExpr = call.Args[0]
			}

		case "Response":
			if len(call.Args) > 0 {
				if code, ok := s.resolveIntValue(call.Args[0]); ok {
					metadata.Responses = append(metadata.Responses, s.parseResponseOptions(code, call.Args[1:]))
				}
			}
		case "AddResponse":
			// AddResponse(code, x) means a schema; AddResponse(code, "text")
			// means a description.
			if len(call.Args) == 2 {
				if code, ok := s.resolveIntValue(call.Args[0]); ok {
					override := meta.ResponseOverride{Code: code}
					if str, isStr := s.resolveStringValue(call.Args[1]); isStr {
						override.Description = str
					} else {
						override.ContentExpr = call.Args[1]
					}
					metadata.Responses = append(metadata.Responses, override)
				}
			}

		case "QueryParam":
			metadata.Parameters = append(metadata.Parameters, s.parseParam("query", call))
		case "PathParam":
			p := s.parseParam("path", call)
			p.Required = true
			metadata.Parameters = append(metadata.Parameters, p)
		case "HeaderParam":
			metadata.Parameters = append(metadata.Parameters, s.parseParam("header", call))
		case "CookieParam":
			metadata.Parameters = append(metadata.Parameters, s.parseParam("cookie", call))

		case "ResponseHeader":
			if len(call.Args) >= 2 {
				code, _ := s.resolveIntValue(call.Args[0])
				name, _ := s.resolveStringValue(call.Args[1])
				header := meta.ResponseHeaderOverride{Code: code, Name: name, Schema: "string"}
				for _, opt := range call.Args[2:] {
					switch optName, optArgs := s.optionCall(opt); optName {
					case "HeaderDoc":
						if len(optArgs) > 0 {
							header.Description, _ = s.resolveStringValue(optArgs[0])
						}
					case "HeaderType":
						if len(optArgs) > 0 {
							header.Schema, _ = s.resolveStringValue(optArgs[0])
						}
					}
				}
				metadata.ResponseHeaders = append(metadata.ResponseHeaders, header)
			}

		case "AddServer":
			if len(call.Args) == 2 {
				url, _ := s.resolveStringValue(call.Args[0])
				desc, _ := s.resolveStringValue(call.Args[1])
				metadata.Servers = append(metadata.Servers, meta.ServerOverride{URL: url, Description: desc})
			}
		case "ExternalDocs":
			if len(call.Args) == 2 {
				url, _ := s.resolveStringValue(call.Args[0])
				desc, _ := s.resolveStringValue(call.Args[1])
				metadata.ExternalDocs = &meta.ExternalDocsOverride{URL: url, Description: desc}
			}
		case "Extensions":
			if len(call.Args) == 1 {
				s.parseExtensions(metadata, call.Args[0])
			}
		case "OperationID":
			if len(call.Args) > 0 {
				if str, ok := s.resolveStringValue(call.Args[0]); ok {
					metadata.OperationID = str
				}
			}
		case "Deprecated":
			metadata.Deprecated = true
		case "Deprecate":
			if len(call.Args) > 0 {
				if val, ok := getBoolValue(call.Args[0]); ok {
					metadata.Deprecated = val
				}
			}
		}
	}

	if handlerExpr == nil {
		return nil, nil
	}
	return metadata, handlerExpr
}

// parseResponseOptions reads respec.Schema and respec.Desc options.
func (s *State) parseResponseOptions(code int, args []ast.Expr) meta.ResponseOverride {
	override := meta.ResponseOverride{Code: code}
	for _, arg := range args {
		switch name, optArgs := s.optionCall(arg); name {
		case "Schema":
			if len(optArgs) > 0 {
				override.ContentExpr = optArgs[0]
			}
		case "Desc":
			if len(optArgs) > 0 {
				override.Description, _ = s.resolveStringValue(optArgs[0])
			}
		}
	}
	return override
}

// parseParam reads a QueryParam/PathParam/HeaderParam/CookieParam call.
func (s *State) parseParam(in string, call *ast.CallExpr) meta.ParameterOverride {
	param := meta.ParameterOverride{In: in, Schema: "string"}
	if len(call.Args) > 0 {
		param.Name, _ = s.resolveStringValue(call.Args[0])
	}
	for _, opt := range call.Args[min(1, len(call.Args)):] {
		switch name, optArgs := s.optionCall(opt); name {
		case "Required":
			param.Required = true
		case "DeprecatedParam":
			param.Deprecated = true
		case "Doc":
			if len(optArgs) > 0 {
				param.Description, _ = s.resolveStringValue(optArgs[0])
			}
		case "Type":
			if len(optArgs) > 0 {
				param.Schema, _ = s.resolveStringValue(optArgs[0])
			}
		case "Example":
			if len(optArgs) > 0 {
				param.Example, _ = s.resolveStringValue(optArgs[0])
			}
		}
	}
	return param
}

// optionCall unwraps a `respec.Xxx(...)` option into its name and arguments.
func (s *State) optionCall(expr ast.Expr) (string, []ast.Expr) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return "", nil
	}
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fun.Sel.Name, call.Args
	case *ast.Ident:
		return fun.Name, call.Args
	}
	return "", nil
}

// parseExtensions reads a map literal of vendor extension fields.
func (s *State) parseExtensions(metadata *meta.HandlerMetadata, arg ast.Expr) {
	if metadata.Extensions == nil {
		metadata.Extensions = make(map[string]any)
	}
	ext, ok := arg.(*ast.CompositeLit)
	if !ok {
		return
	}
	for _, elt := range ext.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		var key string
		switch k := kv.Key.(type) {
		case *ast.Ident:
			key = k.Name
		case *ast.BasicLit:
			if k.Kind == token.STRING {
				if unquoted, err := strconv.Unquote(k.Value); err == nil {
					key = unquoted
				}
			}
		}
		if key == "" {
			continue
		}

		switch {
		case isStringLiteral(kv.Value):
			str, _ := s.resolveStringValue(kv.Value)
			metadata.Extensions[key] = str
		default:
			if i, ok := s.resolveIntValue(kv.Value); ok {
				metadata.Extensions[key] = i
			} else if b, ok := getBoolValue(kv.Value); ok {
				metadata.Extensions[key] = b
			} else if f, ok := s.resolveFloatValue(kv.Value); ok {
				metadata.Extensions[key] = f
			} else if str, ok := s.resolveStringValue(kv.Value); ok {
				metadata.Extensions[key] = str
			} else {
				// The value is not a compile-time constant, so it cannot be
				// carried into a static spec.
				s.reportf(kv.Value.Pos(), "extension %q: value is not a literal and was skipped", key)
			}
		}
	}
}

func isStringLiteral(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}

// getBoolValue resolves a boolean literal.
func getBoolValue(expr ast.Expr) (bool, bool) {
	if ident, ok := expr.(*ast.Ident); ok {
		if ident.Name == "true" || ident.Name == "false" {
			return ident.Name == "true", true
		}
	}
	return false, false
}

// resolveFloatValue resolves a float literal.
func (s *State) resolveFloatValue(expr ast.Expr) (float64, bool) {
	if basicLit, ok := expr.(*ast.BasicLit); ok && basicLit.Kind == token.FLOAT {
		val, err := strconv.ParseFloat(basicLit.Value, 64)
		if err == nil {
			return val, true
		}
	}
	return 0, false
}
