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
	// A cache to store schemas for types we've already processed to avoid re-computation.
	cache map[types.Type]*openapi3.SchemaRef
	// The final map of named components that will be added to the spec.
	Components map[string]*openapi3.SchemaRef
}

// NewSchemaGenerator returns a new SchemaGenerator instance.
func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		cache:      make(map[types.Type]*openapi3.SchemaRef),
		Components: make(map[string]*openapi3.SchemaRef),
	}
}

// GenerateSchema is the main public entry point for creating a schema from a Go type.
func (sg *SchemaGenerator) GenerateSchema(t types.Type) *openapi3.SchemaRef {
	// If we have already processed this exact type, return the cached version.
	if ref, ok := sg.cache[t]; ok {
		return ref
	}

	// The core logic is now in buildSchemaRef.
	schemaRef := sg.buildSchemaRef(t)

	// Cache the result for this type to handle recursion and avoid re-work.
	sg.cache[t] = schemaRef
	return schemaRef
}

// buildSchemaRef is the main dispatcher. It correctly decides whether to create a reusable
// component with a $ref, or an inline schema definition.
func (sg *SchemaGenerator) buildSchemaRef(t types.Type) *openapi3.SchemaRef {
	switch u := t.(type) {
	case *types.Named:
		// This is a named type. We must check its underlying type.
		underlying := u.Underlying()

		// Case 1: The underlying type is a struct. This is a standard, named struct
		// that should become a reusable component.
		if s, isStruct := underlying.(*types.Struct); isStruct {
			componentName := u.Obj().Name()
			if componentName == "" {
				// This should not happen for a valid named type, but as a safeguard,
				// treat it as an anonymous struct and define it inline.
				return &openapi3.SchemaRef{Value: sg.schemaForStruct(s)}
			}

			refPath := "#/components/schemas/" + componentName

			// If this component is already being processed, we've hit a recursive loop.
			// Return the reference to the placeholder that has already been created.
			if _, ok := sg.Components[componentName]; ok {
				return openapi3.NewSchemaRef(refPath, nil)
			}

			// Create a placeholder Schema. This will be the value for our component.
			placeholderSchema := &openapi3.Schema{}
			sg.Components[componentName] = &openapi3.SchemaRef{Value: placeholderSchema}

			// Create a reference to this new component and cache it.
			ref := openapi3.NewSchemaRef(refPath, nil)
			sg.cache[t] = ref

			// Now, build the actual schema properties and populate the placeholder.
			*placeholderSchema = *sg.schemaForStruct(s)
			return ref
		} else {
			// Case 2: It's a named type but not a struct (e.g., type UserID string).
			// We should not create a component for it, but instead use the schema
			// for its underlying basic type (e.g., 'string').
			return sg.GenerateSchema(underlying)
		}

	case *types.Pointer:
		return sg.GenerateSchema(u.Elem())
	case *types.Slice:
		schema := openapi3.NewArraySchema()
		schema.Items = sg.GenerateSchema(u.Elem())
		return &openapi3.SchemaRef{Value: schema}
	case *types.Map:
		schema := openapi3.NewObjectSchema()
		schema.AdditionalProperties = openapi3.AdditionalProperties{Schema: sg.GenerateSchema(u.Elem())}
		return &openapi3.SchemaRef{Value: schema}
	case *types.Struct:
		// Case 3: This is an anonymous struct. It must be defined inline.
		return &openapi3.SchemaRef{Value: sg.schemaForStruct(u)}
	case *types.Basic:
		return &openapi3.SchemaRef{Value: sg.schemaForBasic(u)}
	default:
		schema := openapi3.NewObjectSchema()
		schema.Description = fmt.Sprintf("Unsupported type: %T", u)
		return &openapi3.SchemaRef{Value: schema}
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
		schema := openapi3.NewStringSchema()
		schema.Description = "Type " + b.Name()
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
		tag := s.Tag(i)
		jsonTag := reflect.StructTag(tag).Get("json")
		parts := strings.Split(jsonTag, ",")
		fieldName := parts[0]
		if fieldName == "-" {
			continue
		}
		if fieldName == "" {
			fieldName = field.Name()
		}
		fieldSchemaRef := sg.GenerateSchema(field.Type())
		schema.WithPropertyRef(fieldName, fieldSchemaRef)
	}
	return schema
}
