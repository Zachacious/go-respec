package analyzer

import (
	"fmt"
	"go/types"
	"reflect"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// SchemaGenerator turns Go types into OpenAPI schema definitions.
type SchemaGenerator struct {
	// A cache to store schemas for types we've already processed, avoiding re-computation and handling recursion.
	schemas map[types.Type]*openapi3.SchemaRef
}

func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		schemas: make(map[types.Type]*openapi3.SchemaRef),
	}
}

// GenerateSchema is the main entry point for creating a schema from a Go type.
// It returns a reference to the schema, which will be placed in the components section.
func (sg *SchemaGenerator) GenerateSchema(t types.Type) *openapi3.SchemaRef {
	// If we've seen this type before, return the cached reference.
	if ref, ok := sg.schemas[t]; ok {
		return ref
	}

	// For named types (like structs), use the type's name. For others, generate a name.
	typeName := t.String()
	if named, ok := t.(*types.Named); ok {
		typeName = named.Obj().Name()
	}

	// Create a preliminary reference. This is crucial for handling recursive types.
	// We add it to the cache *before* processing to break cycles.
	schemaRef := &openapi3.SchemaRef{Ref: "#/components/schemas/" + typeName}
	sg.schemas[t] = schemaRef

	// Now, actually build the schema that the reference points to.
	schema := sg.buildSchema(t)
	schemaRef.Value = schema

	return schemaRef
}

// buildSchema does the actual work of converting a type to a schema.
func (sg *SchemaGenerator) buildSchema(t types.Type) *openapi3.Schema {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return sg.schemaForBasic(u)
	case *types.Struct:
		return sg.schemaForStruct(u)
	case *types.Slice:
		return openapi3.NewArraySchema().WithItems(sg.GenerateSchema(u.Elem()).Value)
	case *types.Pointer:
		return sg.GenerateSchema(u.Elem()).Value
	case *types.Map:
		return openapi3.NewObjectSchema().WithAdditionalProperties(sg.GenerateSchema(u.Elem()).Value)
	default:
		// --- Start of fix for error 3 ---
		schema := openapi3.NewObjectSchema()
		schema.Description = fmt.Sprintf("Unsupported type: %T", u)
		return schema
		// --- End of fix ---
	}
}

func (sg *SchemaGenerator) schemaForBasic(b *types.Basic) *openapi3.Schema {
	switch b.Kind() {
	case types.String:
		return openapi3.NewStringSchema()
	case types.Bool:
		return openapi3.NewBoolSchema()
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
		return openapi3.NewIntegerSchema()
	case types.Float32, types.Float64:
		return openapi3.NewFloat64Schema()
	default:
		// --- Start of fix for error 4 ---
		schema := openapi3.NewStringSchema()
		schema.Description = "Type " + b.Name()
		return schema
		// --- End of fix ---
	}
}

func (sg *SchemaGenerator) schemaForStruct(s *types.Struct) *openapi3.Schema {
	schema := openapi3.NewObjectSchema()
	for i := 0; i < s.NumFields(); i++ {
		field := s.Field(i)
		// Ignore unexported fields
		if !field.Exported() {
			continue
		}

		tag := s.Tag(i)
		jsonTag := reflect.StructTag(tag).Get("json")
		parts := strings.Split(jsonTag, ",")
		fieldName := parts[0]

		if fieldName == "-" {
			continue // Field is explicitly ignored
		}
		if fieldName == "" {
			fieldName = field.Name() // Default to field name
		}

		// Recursively generate the schema for the field's type.
		fieldSchemaRef := sg.GenerateSchema(field.Type())
		schema.WithPropertyRef(fieldName, fieldSchemaRef)
	}
	return schema
}
