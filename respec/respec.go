// Package respec provides a fluent API for adding OpenAPI metadata.
package respec

import (
	"fmt"
	"go/types"
	"net/http"
	"reflect"
	"runtime"
	"sync"
)

// --- Public API ---

// Handler wraps an http.HandlerFunc to allow for endpoint-specific metadata.
func Handler(handler http.HandlerFunc) *Builder {
	builder := newBuilder()
	key := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	metadataStore.handlerData.set(key, builder)
	builder.handler = handler
	return builder
}

// Meta provides a way to attach metadata to a router instance within a specific scope.
func Meta(router interface{}) *GroupBuilder {
	// This function is primarily a marker for the static analyzer.
	return NewGroupBuilder()
}

// --- Builder for Handlers (`respec.Handler`) ---

type Builder struct {
	summary     string
	description string
	tags        []string
	security    []string
	handler     http.HandlerFunc
}

func newBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if b.handler != nil {
		b.handler(w, r)
	}
}

func (b *Builder) GetSummary() string     { return b.summary }
func (b *Builder) GetDescription() string { return b.description }
func (b *Builder) GetTags() []string      { return b.tags }
func (b *Builder) GetSecurity() []string  { return b.security }

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

// --- Builder for Groups (`respec.Meta`) ---

type GroupBuilder struct {
	tags     []string
	security []string
}

// NewGroupBuilder is exported for use by the analyzer.
func NewGroupBuilder() *GroupBuilder {
	return &GroupBuilder{}
}
func (b *GroupBuilder) GetTags() []string     { return b.tags }
func (b *GroupBuilder) GetSecurity() []string { return b.security }
func (b *GroupBuilder) Tag(tags ...string) *GroupBuilder {
	b.tags = append(b.tags, tags...)
	return b
}
func (b *GroupBuilder) Security(schemeName string) *GroupBuilder {
	b.security = append(b.security, schemeName)
	return b
}

// --- Internal Metadata Store ---

var metadataStore = struct {
	handlerData *handlerStore
}{
	handlerData: newHandlerStore(),
}

type handlerStore struct {
	sync.RWMutex
	data map[string]*Builder
}

func newHandlerStore() *handlerStore {
	return &handlerStore{data: make(map[string]*Builder)}
}
func (s *handlerStore) set(key string, b *Builder) {
	s.Lock()
	defer s.Unlock()
	s.data[key] = b
}
func (s *handlerStore) get(key string) *Builder {
	s.RLock()
	defer s.RUnlock()
	return s.data[key]
}

func GetByHandler(handler types.Object) *Builder {
	key := getFuncKeyFromTypes(handler)
	if key == "" {
		return nil
	}
	return metadataStore.handlerData.get(key)
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
