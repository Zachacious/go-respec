package analyzer

import (
	"fmt"
	"go/types"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// SchemaGenerator turns Go types into OpenAPI schema definitions.
type SchemaGenerator struct {
	// A cache to store schemas for types we've already processed to avoid re-computation.
	cache map[types.Type]*openapi3.SchemaRef
	// The final map of named components that will be added to the spec.
	Components map[string]*openapi3.SchemaRef
	// componentOwner records which named type claimed each component name, so a
	// second type with the same name can be detected rather than silently
	// reusing the first one's schema.
	componentOwner map[string]*types.Named
	// contestedNames are bare type names that two or more packages wanted. They
	// stay reserved so no later type can claim the unqualified spelling.
	contestedNames map[string]bool
}

// NewSchemaGenerator returns a new SchemaGenerator instance.
func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		cache:          make(map[types.Type]*openapi3.SchemaRef),
		Components:     make(map[string]*openapi3.SchemaRef),
		componentOwner: make(map[string]*types.Named),
		contestedNames: make(map[string]bool),
	}
}

// componentNameFor picks a unique component name for a named type.
//
// Bare type names collide across packages, and the collision was silent: the
// second admin.User to arrive would reuse public.User's component, so an
// endpoint ended up documenting fields it never returns. On a conflict both
// types get a package-qualified name so neither is misrepresented.
func (sg *SchemaGenerator) componentNameFor(named *types.Named) string {
	base := baseName(named)

	// Once a bare name has been contested it stays reserved forever. Freeing it
	// would let a third type with the same name claim the unqualified spelling
	// while its two siblings stayed qualified, which reads as though that one
	// were canonical.
	if sg.contestedNames[base] {
		return sg.uniqueQualifiedName(named)
	}

	owner, taken := sg.componentOwner[base]
	if !taken {
		sg.componentOwner[base] = named
		return base
	}
	if owner == named || types.Identical(owner, named) {
		return base
	}

	// Two distinct types want the same name. Qualify this one, and retroactively
	// qualify the incumbent so the plain name never refers to just one of them.
	if schema, ok := sg.Components[base]; ok {
		ownerQualified := qualifiedName(owner)
		sg.Components[ownerQualified] = schema
		delete(sg.Components, base)
		sg.componentOwner[ownerQualified] = owner
		delete(sg.componentOwner, base)

		// Refs handed out earlier are already embedded in operations that have
		// been built, so the pointer has to be rewritten in place. Swapping in a
		// new SchemaRef would leave those operations pointing at a component
		// name that no longer exists.
		oldPath := "#/components/schemas/" + base
		newPath := "#/components/schemas/" + ownerQualified
		for _, ref := range sg.cache {
			if ref != nil && ref.Ref == oldPath {
				ref.Ref = newPath
			}
		}
	}

	sg.contestedNames[base] = true
	return sg.uniqueQualifiedName(named)
}

// uniqueQualifiedName returns a package-qualified name, appending a counter in
// the event that two packages share a base name as well as a type name.
func (sg *SchemaGenerator) uniqueQualifiedName(named *types.Named) string {
	name := qualifiedName(named)
	if owner, taken := sg.componentOwner[name]; !taken || owner == named {
		sg.componentOwner[name] = named
		return name
	}

	for i := 2; ; i++ {
		candidate := name + "_" + strconv.Itoa(i)
		if owner, taken := sg.componentOwner[candidate]; !taken || owner == named {
			sg.componentOwner[candidate] = named
			return candidate
		}
	}
}

// qualifiedName renders a type as Package_Type, which is unique in practice and
// still readable in generated documentation.
func qualifiedName(named *types.Named) string {
	obj := named.Obj()
	if obj.Pkg() == nil {
		return sanitizeComponentName(baseName(named))
	}
	return sanitizeComponentName(obj.Pkg().Name() + "_" + baseName(named))
}

// baseName is a type's name with its type arguments appended.
//
// Every instantiation of a generic type otherwise reports the same name, so a
// codebase using Response[User] and Response[Order] - a common way to wrap API
// payloads - would see one collapse into the other.
func baseName(named *types.Named) string {
	name := named.Obj().Name()

	args := named.TypeArgs()
	if args == nil || args.Len() == 0 {
		return name
	}

	var b strings.Builder
	b.WriteString(name)
	for i := range args.Len() {
		b.WriteString("_")
		b.WriteString(typeArgName(args.At(i)))
	}
	return b.String()
}

// typeArgName renders a type argument compactly for use in a component name.
func typeArgName(t types.Type) string {
	switch u := types.Unalias(t).(type) {
	case *types.Named:
		return baseName(u)
	case *types.Pointer:
		return typeArgName(u.Elem())
	case *types.Slice:
		return typeArgName(u.Elem()) + "List"
	case *types.Basic:
		return u.Name()
	default:
		return "T"
	}
}

// sanitizeComponentName strips characters that are not valid in a component key.
func sanitizeComponentName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '.', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// GenerateSchema is the main public entry point for creating a schema from a Go type.
func (sg *SchemaGenerator) GenerateSchema(t types.Type) *openapi3.SchemaRef {
	// `any` is an alias for interface{}, and since Go 1.23 aliases are their own
	// node in the type graph. Without unaliasing, every `any` field rendered as
	// "Unsupported type: *types.Alias".
	t = types.Unalias(t)

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
		// Some standard types have a well-known JSON form that bears no
		// relation to their Go struct. time.Time in particular has no exported
		// fields, so treating it structurally produced an empty object.
		if schema := wellKnownSchema(u); schema != nil {
			return &openapi3.SchemaRef{Value: schema}
		}

		underlying := u.Underlying()

		// Case 1: The underlying type is a struct. This is a standard, named struct
		// that should become a reusable component.
		if s, isStruct := underlying.(*types.Struct); isStruct {
			if u.Obj().Name() == "" {
				// This should not happen for a valid named type, but as a safeguard,
				// treat it as an anonymous struct and define it inline.
				return &openapi3.SchemaRef{Value: sg.schemaForStruct(s)}
			}

			componentName := sg.componentNameFor(u)
			refPath := "#/components/schemas/" + componentName

			// Publish a placeholder and cache the ref before building the
			// properties, so a type that refers to itself resolves to this ref
			// instead of recursing forever.
			placeholderSchema := &openapi3.Schema{}
			sg.Components[componentName] = &openapi3.SchemaRef{Value: placeholderSchema}
			ref := openapi3.NewSchemaRef(refPath, nil)
			sg.cache[t] = ref

			*placeholderSchema = *sg.schemaForStruct(s)
			return ref
		}
		// Case 2: It's a named type but not a struct (e.g., type UserID string).
		// We should not create a component for it, but instead use the schema
		// for its underlying basic type (e.g., 'string').
		return sg.GenerateSchema(underlying)

	case *types.Pointer:
		// Whether a pointer is nullable depends on the field's json tag, not on
		// the type, so the decision is made per-field in addStructFields. Here
		// the pointer is simply transparent.
		return sg.GenerateSchema(u.Elem())
	case *types.Slice:
		// []byte marshals to a base64 string, not to an array of numbers.
		if basic, ok := u.Elem().Underlying().(*types.Basic); ok && basic.Kind() == types.Uint8 {
			schema := openapi3.NewStringSchema()
			schema.Format = "byte"
			return &openapi3.SchemaRef{Value: schema}
		}
		schema := openapi3.NewArraySchema()
		schema.Items = sg.GenerateSchema(u.Elem())
		return &openapi3.SchemaRef{Value: schema}
	case *types.Array:
		schema := openapi3.NewArraySchema()
		schema.Items = sg.GenerateSchema(u.Elem())
		return &openapi3.SchemaRef{Value: schema}
	case *types.Interface:
		// An `any` field accepts anything; an empty schema says exactly that.
		return &openapi3.SchemaRef{Value: &openapi3.Schema{}}
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

// wellKnownSchema returns the JSON shape of types whose Go structure says
// nothing useful about how they serialise. Returns nil for everything else.
func wellKnownSchema(named *types.Named) *openapi3.Schema {
	obj := named.Obj()
	if obj.Pkg() == nil {
		return nil
	}

	switch obj.Pkg().Path() + "." + obj.Name() {
	case "time.Time":
		schema := openapi3.NewStringSchema()
		schema.Format = "date-time"
		return schema
	case "time.Duration":
		schema := openapi3.NewIntegerSchema()
		schema.Format = "int64"
		schema.Description = "Duration in nanoseconds"
		return schema
	case "github.com/google/uuid.UUID", "github.com/gofrs/uuid.UUID":
		schema := openapi3.NewStringSchema()
		schema.Format = "uuid"
		return schema
	case "encoding/json.RawMessage":
		return &openapi3.Schema{}
	case "math/big.Int", "math/big.Float":
		schema := openapi3.NewStringSchema()
		schema.Format = "number"
		return schema
	case "net/url.URL":
		schema := openapi3.NewStringSchema()
		schema.Format = "uri"
		return schema
	}
	return nil
}

// nullableRef marks a schema as accepting null.
//
// In OpenAPI 3.1 nullability is expressed by adding "null" to the type list, so
// the referenced schema is wrapped rather than mutated - mutating it would make
// every other use of that component nullable too.
func nullableRef(ref *openapi3.SchemaRef) *openapi3.SchemaRef {
	if ref == nil {
		return nil
	}
	// A $ref cannot carry sibling keywords, so wrap it in a one-element anyOf
	// alongside an explicit null.
	if ref.Ref != "" {
		return &openapi3.SchemaRef{Value: &openapi3.Schema{
			AnyOf: openapi3.SchemaRefs{
				ref,
				{Value: &openapi3.Schema{Type: &openapi3.Types{"null"}}},
			},
		}}
	}
	if ref.Value == nil || ref.Value.Type == nil {
		return ref
	}
	types := append(openapi3.Types{}, *ref.Value.Type...)
	if !slices.Contains(types, "null") {
		types = append(types, "null")
	}
	clone := *ref.Value
	clone.Type = &types
	return &openapi3.SchemaRef{Value: &clone}
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
	sg.addStructFields(schema, s)

	// Keep required deterministic; property order is a map and already sorted
	// on marshal, but required is a slice.
	sort.Strings(schema.Required)
	return schema
}

// addStructFields populates a schema from a struct, following Go's own JSON
// rules for embedded types.
func (sg *SchemaGenerator) addStructFields(schema *openapi3.Schema, s *types.Struct) {
	for i := range s.NumFields() {
		field := s.Field(i)
		tag := reflect.StructTag(s.Tag(i))
		jsonTag := tag.Get("json")
		parts := strings.Split(jsonTag, ",")
		nameOverride := parts[0]

		if nameOverride == "-" && len(parts) == 1 {
			continue
		}

		// An embedded struct with no JSON name is inlined by encoding/json, so
		// its fields belong to the parent object rather than to a nested one.
		if field.Embedded() && nameOverride == "" {
			if embedded, ok := underlyingStruct(field.Type()); ok {
				sg.addStructFields(schema, embedded)
				continue
			}
		}

		if !field.Exported() {
			continue
		}

		fieldName := nameOverride
		if fieldName == "" {
			fieldName = field.Name()
		}

		fieldSchema := sg.GenerateSchema(field.Type())

		// encoding/json decides presence and nullability together, and a
		// pointer means different things depending on the tag:
		//
		//   *T `json:"f,omitempty"`  nil -> the key is absent. Optional, never null.
		//   *T `json:"f"`            nil -> "f": null. Always present, and nullable.
		//
		// Treating every pointer as nullable, as respec did, described a shape
		// the API never actually produces, and buried the genuinely nullable
		// fields in noise.
		omitEmpty := slices.Contains(parts[1:], "omitempty")
		if isPointer(field.Type()) && !omitEmpty {
			fieldSchema = nullableRef(fieldSchema)
		}

		if desc := tag.Get("description"); desc != "" && fieldSchema != nil && fieldSchema.Value != nil {
			fieldSchema.Value.Description = desc
		}
		schema.WithPropertyRef(fieldName, fieldSchema)

		if !omitEmpty {
			schema.Required = append(schema.Required, fieldName)
		}
	}
}

func underlyingStruct(t types.Type) (*types.Struct, bool) {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	s, ok := t.Underlying().(*types.Struct)
	return s, ok
}

func isPointer(t types.Type) bool {
	_, ok := t.(*types.Pointer)
	return ok
}
