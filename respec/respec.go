// Package respec provides a fluent API for adding OpenAPI metadata to http.HandlerFuncs.
package respec

import (
	"fmt"
	"go/types"
	"net/http"
	"reflect"
	"runtime"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
)

// --- Public API ---

// Handler wraps an http.HandlerFunc to allow for metadata decoration.
// This is the main entry point for developers adding endpoint-specific metadata.
// Usage: r.Post("/users", respec.Handler(myHandler).Tag("Users"))
func Handler(handler http.HandlerFunc) *Builder {
	builder := newBuilder()
	key := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	metadataStore.set(key, builder)

	builder.handler = handler
	return builder
}

// Group is a marker function used to apply metadata to a block of routes.
// At runtime, it simply executes the provided function to register the routes.
// Its primary purpose is to serve as a detectable anchor for the static analyzer.
func Group(groupFunc func()) *GroupBuilder {
	builder := newGroupBuilder()
	// Execute the user's function to ensure routes are registered.
	groupFunc()
	return builder
}

// --- Builder for Handlers ---

type Builder struct {
	summary        string
	description    string
	tags           []string
	security       []string
	paramModifiers map[string]ParamModifier
	handler        http.HandlerFunc
}

func newBuilder() *Builder {
	return &Builder{
		paramModifiers: make(map[string]ParamModifier),
	}
}

func (b *Builder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if b.handler != nil {
		b.handler(w, r)
	}
}

func (b *Builder) GetSummary() string                          { return b.summary }
func (b *Builder) GetDescription() string                      { return b.description }
func (b *Builder) GetTags() []string                           { return b.tags }
func (b *Builder) GetSecurity() []string                       { return b.security }
func (b *Builder) GetParamModifiers() map[string]ParamModifier { return b.paramModifiers }

func (b *Builder) Summary(s string) *Builder {
	b.summary = s
	return b
}

func (b *Builder) Description(d string) *Builder {
	b.description = d
	return b
}

func (b *Builder) Tag(tags ...string) *Builder {
	b.tags = append(b.tags, tags...)
	return b
}

func (b *Builder) Security(schemeName string) *Builder {
	b.security = append(b.security, schemeName)
	return b
}

func (b *Builder) OverrideParam(name string, modifier ParamModifier) *Builder {
	b.paramModifiers[name] = modifier
	return b
}

// --- Builder for Groups ---

type GroupBuilder struct {
	tags     []string
	security []string
}

func newGroupBuilder() *GroupBuilder {
	return &GroupBuilder{}
}

func (b *GroupBuilder) Tag(tags ...string) *GroupBuilder {
	b.tags = append(b.tags, tags...)
	return b
}

func (b *GroupBuilder) Security(schemeName string) *GroupBuilder {
	b.security = append(b.security, schemeName)
	return b
}

// --- Internal Logic ---

type ParamModifier func(p *openapi3.Parameter)
type ResponseModifier func(r *openapi3.Response)

var metadataStore = newStore()

type store struct {
	sync.RWMutex
	data map[string]*Builder
}

func newStore() *store {
	return &store{
		data: make(map[string]*Builder),
	}
}

func (s *store) set(key string, b *Builder) {
	s.Lock()
	defer s.Unlock()
	s.data[key] = b
}

func GetByHandler(handler types.Object) *Builder {
	key := getFuncKeyFromTypes(handler)
	if key == "" {
		return nil
	}
	metadataStore.RLock()
	defer metadataStore.RUnlock()
	return metadataStore.data[key]
}

func getFuncKeyFromTypes(obj types.Object) string {
	if obj == nil {
		return ""
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		return ""
	}
	if sig := fn.Type().(*types.Signature); sig != nil && sig.Recv() != nil {
		return fmt.Sprintf("%s.%s", sig.Recv().Type().String(), fn.Name())
	}
	if fn.Pkg() == nil {
		return ""
	}
	return fmt.Sprintf("%s.%s", fn.Pkg().Path(), fn.Name())
}
