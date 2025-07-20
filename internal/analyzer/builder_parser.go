package analyzer

import (
	"go/ast"
	"go/token"
	"strconv"

	"github.com/Zachacious/go-respec/respec"
)

// FindAndParseRouteMetadata scans the AST for `respec.Handler(...).Unwrap()` call chains.
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

				if handlerObj := s.getObjectForExpr(handlerExpr); handlerObj != nil {
					s.OperationMetadata[handlerObj] = metadata
				}
				return false
			})
		}
	}
}

// parseHandlerChain walks a call chain backwards to parse metadata.
func (s *State) parseHandlerChain(expr ast.Expr) (*respec.HandlerMetadata, ast.Expr) {
	metadata := &respec.HandlerMetadata{}
	currentExpr := expr

	for {
		call, isCall := currentExpr.(*ast.CallExpr)
		if !isCall {
			break
		}
		sel, isSel := call.Fun.(*ast.SelectorExpr)
		if !isSel {
			break
		}

		methodName := sel.Sel.Name

		switch methodName {
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
		case "RequestBody":
			if len(call.Args) > 0 {
				metadata.RequestBodyExpr = call.Args[0]
			}
		case "AddResponse":
			if len(call.Args) == 2 {
				if code, ok := s.resolveIntValue(call.Args[0]); ok {
					metadata.Responses = append(metadata.Responses, respec.ResponseOverride{Code: code, ContentExpr: call.Args[1]})
				}
			}
		case "AddParameter":
			if len(call.Args) == 5 {
				in, _ := s.resolveStringValue(call.Args[0])
				name, _ := s.resolveStringValue(call.Args[1])
				desc, _ := s.resolveStringValue(call.Args[2])
				req, _ := getBoolValue(call.Args[3])
				dep, _ := getBoolValue(call.Args[4])
				metadata.Parameters = append(metadata.Parameters, respec.ParameterOverride{In: in, Name: name, Description: desc, Required: req, Deprecated: dep})
			}
		case "ResponseHeader":
			if len(call.Args) == 3 {
				code, _ := s.resolveIntValue(call.Args[0])
				name, _ := s.resolveStringValue(call.Args[1])
				desc, _ := s.resolveStringValue(call.Args[2])
				metadata.ResponseHeaders = append(metadata.ResponseHeaders, respec.ResponseHeaderOverride{Code: code, Name: name, Description: desc})
			}
		case "AddServer":
			if len(call.Args) == 2 {
				url, _ := s.resolveStringValue(call.Args[0])
				desc, _ := s.resolveStringValue(call.Args[1])
				metadata.Servers = append(metadata.Servers, respec.ServerOverride{URL: url, Description: desc})
			}
		case "ExternalDocs":
			if len(call.Args) == 2 {
				url, _ := s.resolveStringValue(call.Args[0])
				desc, _ := s.resolveStringValue(call.Args[1])
				metadata.ExternalDocs = &respec.ExternalDocsOverride{URL: url, Description: desc}
			}

		case "Extensions":
			if len(call.Args) == 1 {
				// Ensure Extensions map is initialized
				if metadata.Extensions == nil {
					metadata.Extensions = make(map[string]any)
				}
				if ext, ok := call.Args[0].(*ast.CompositeLit); ok {
					for _, elt := range ext.Elts {
						kv, ok := elt.(*ast.KeyValueExpr)
						if !ok {
							continue
						}

						// Resolve key: support string literals or idents
						var key string
						if kIdent, ok := kv.Key.(*ast.Ident); ok {
							key = kIdent.Name
						} else if kBasicLit, ok := kv.Key.(*ast.BasicLit); ok {
							if kBasicLit.Kind == token.STRING {
								if k, err := strconv.Unquote(kBasicLit.Value); err == nil {
									key = k
								}
							}
						}

						if key == "" {
							continue
						}

						// Resolve value: try string, int, bool, float
						if str, ok := s.resolveStringValue(kv.Value); ok {
							metadata.Extensions[key] = str
						} else if i, ok := s.resolveIntValue(kv.Value); ok {
							metadata.Extensions[key] = i
						} else if b, ok := getBoolValue(kv.Value); ok {
							metadata.Extensions[key] = b
						} else if f, ok := s.resolveFloatValue(kv.Value); ok {
							metadata.Extensions[key] = f
						} else {
							// fallback: store raw expression?
							metadata.Extensions[key] = kv.Value
						}
					}
				}
			}

		case "OperationID":
			if len(call.Args) > 0 {
				if str, ok := s.resolveStringValue(call.Args[0]); ok {
					metadata.OperationID = str
				}
			}
		case "Deprecate":
			if len(call.Args) > 0 {
				if val, ok := getBoolValue(call.Args[0]); ok {
					metadata.Deprecated = val
				}
			}
		case "Handler":
			if len(call.Args) == 1 {
				return metadata, call.Args[0]
			}
			return nil, nil
		}
		currentExpr = sel.X
	}
	return nil, nil
}

// getBoolValue is a simple helper to resolve a boolean literal.
func getBoolValue(expr ast.Expr) (bool, bool) {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == "true", true
	}
	return false, false
}

// resolve float value from an expression.
func (s *State) resolveFloatValue(expr ast.Expr) (float64, bool) {
	if basicLit, ok := expr.(*ast.BasicLit); ok && basicLit.Kind == token.FLOAT {
		val, err := strconv.ParseFloat(basicLit.Value, 64)
		if err == nil {
			return val, true
		}
	}
	return 0, false
}
