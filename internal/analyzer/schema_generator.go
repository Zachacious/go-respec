package analyzer

import (
	"go/types"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// SchemaGenerator holds state for schema generation, like a cache for already-generated types.
type SchemaGenerator struct {
	schemas map[string]*openapi3.SchemaRef
}

func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		schemas: make(map[string]*openapi3.SchemaRef),
	}
}

// GenerateSchemaRef creates a JSON Schema for a given Go type and adds it to the component map.
// It returns a reference to the schema.
func (sg *SchemaGenerator) GenerateSchemaRef(typeObj types.Type) *openapi3.SchemaRef {
	typeName := typeObj.String() // A simplification; a robust version uses package path + name

	// If we've already generated this schema, return a reference to it.
	if ref, ok := sg.schemas[typeName]; ok {
		return ref
	}

	schema := sg.generateSchema(typeObj)

	// For named types (structs), we store them in the components and create a reference.
	if named, ok := typeObj.(*types.Named); ok {
		// Use the actual type name for the schema key
		schemaName := named.Obj().Name()
		sg.schemas[schemaName] = &openapi3.SchemaRef{
			Value: schema,
		}
		return &openapi3.SchemaRef{
			Ref: "#/components/schemas/" + schemaName,
		}
	}

	// For anonymous types, return the schema directly.
	return &openapi3.SchemaRef{
		Value: schema,
	}
}

// generateSchema does the core work of converting a Go type to an OpenAPI schema.
func (sg *SchemaGenerator) generateSchema(typeObj types.Type) *openapi3.Schema {
	if ptr, ok := typeObj.(*types.Pointer); ok {
		typeObj = ptr.Elem()
	}

	switch t := typeObj.Underlying().(type) {
	case *types.Basic:
		return sg.schemaForBasic(t)
	case *types.Struct:
		return sg.schemaForStruct(t)
	case *types.Slice:
		return openapi3.NewArraySchema().WithItems(sg.generateSchema(t.Elem()))
	case *types.Map:
		return openapi3.NewObjectSchema()
	default:
		// FIX: Direct assignment for Description
		schema := openapi3.NewObjectSchema()
		schema.Description = "Unsupported type: " + t.String()
		return schema
	}
}

func (sg *SchemaGenerator) schemaForBasic(basic *types.Basic) *openapi3.Schema {
	switch basic.Kind() {
	case types.String:
		return openapi3.NewStringSchema()
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
		return openapi3.NewIntegerSchema()
	case types.Float32, types.Float64:
		return openapi3.NewFloat64Schema()
	case types.Bool:
		return openapi3.NewBoolSchema()
	default:
		// FIX: Direct assignment for Description
		schema := openapi3.NewStringSchema()
		schema.Description = "Type mapped to string"
		return schema
	}
}

func (sg *SchemaGenerator) schemaForStruct(s *types.Struct) *openapi3.Schema {
	schema := openapi3.NewObjectSchema()
	schema.Properties = make(map[string]*openapi3.SchemaRef)

	for i := 0; i < s.NumFields(); i++ {
		field := s.Field(i)

		// Skip unexported fields
		if !field.Exported() {
			continue
		}

		jsonTag := s.Tag(i)
		jsonName := parseJsonTag(jsonTag)
		if jsonName == "" {
			jsonName = field.Name() // Default to field name
		}
		if jsonName == "-" {
			continue // Field is ignored by JSON marshaller
		}

		schema.Properties[jsonName] = sg.GenerateSchemaRef(field.Type())
	}
	return schema
}

func parseJsonTag(tag string) string {
	// A real implementation would parse the whole tag `json:"name,omitempty"`
	// This is a simplified version.
	if strings.Contains(tag, `json:"`) {
		start := strings.Index(tag, `json:"`) + len(`json:"`)
		end := strings.Index(tag[start:], `"`)
		return tag[start : start+end]
	}
	return ""
}
