package analyzer

import (
	"go/types"
	"reflect"
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
	// Follow pointers and get the underlying type, but store the original for naming.
	originalType := typeObj
	if ptr, ok := typeObj.(*types.Pointer); ok {
		typeObj = ptr.Elem()
	}

	// Use the qualified type name for caching to avoid conflicts.
	var typeName string
	if named, ok := originalType.(*types.Named); ok {
		typeName = named.Obj().Pkg().Path() + "." + named.Obj().Name()
	} else {
		typeName = originalType.String()
	}

	if ref, ok := sg.schemas[typeName]; ok {
		return ref
	}

	schema := sg.generateSchema(typeObj)

	if named, ok := typeObj.(*types.Named); ok {
		schemaName := named.Obj().Name()
		// Store the full schema by its proper name in components
		sg.schemas[schemaName] = &openapi3.SchemaRef{
			Value: schema,
		}
		// Return a reference to it
		return openapi3.NewSchemaRef("#/components/schemas/"+schemaName, nil)
	}

	// For anonymous types, return the schema directly.
	return &openapi3.SchemaRef{Value: schema}
}

// generateSchema does the core work of converting a Go type to an OpenAPI schema.
func (sg *SchemaGenerator) generateSchema(typeObj types.Type) *openapi3.Schema {
	switch t := typeObj.Underlying().(type) {
	case *types.Basic:
		return sg.schemaForBasic(t)
	case *types.Struct:
		return sg.schemaForStruct(t)
	case *types.Slice:
		// FIX: The WithItems method expects a *Schema, not a *SchemaRef.
		// The correct way is to assign the SchemaRef to the Items field directly.
		schema := openapi3.NewArraySchema()
		schema.Items = sg.GenerateSchemaRef(t.Elem())
		return schema
	case *types.Map:
		schema := openapi3.NewObjectSchema()
		schema.AdditionalProperties = openapi3.AdditionalProperties{
			Schema: sg.GenerateSchemaRef(t.Elem()),
		}
		return schema
	default:
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
		if !field.Exported() {
			continue
		}

		jsonTag := s.Tag(i)
		jsonName := parseJsonTag(jsonTag)
		if jsonName == "" {
			jsonName = field.Name()
		}
		if jsonName == "-" {
			continue
		}

		schema.Properties[jsonName] = sg.GenerateSchemaRef(field.Type())
	}
	return schema
}

func parseJsonTag(tag string) string {
	// FIX: Use reflect.StructTag to correctly parse the tag.
	tagValue := reflect.StructTag(tag).Get("json")
	parts := strings.Split(tagValue, ",")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}
