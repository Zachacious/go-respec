package analyzer

import (
	"testing"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
)

func TestSplitMethodPattern(t *testing.T) {
	tests := []struct {
		in         string
		wantMethod string
		wantPath   string
	}{
		{"GET /users", "GET", "/users"},
		{"POST /users/{id}", "POST", "/users/{id}"},
		{"get /users", "GET", "/users"},
		// No verb means the pattern matches every method.
		{"/users", "", "/users"},
		{"/users/{id}", "", "/users/{id}"},
		// A leading token that is not a verb belongs to the path, not the method.
		{"example.com/users", "", "example.com/users"},
	}

	for _, tt := range tests {
		gotMethod, gotPath := splitMethodPattern(tt.in)
		if gotMethod != tt.wantMethod || gotPath != tt.wantPath {
			t.Errorf("splitMethodPattern(%q) = (%q, %q), want (%q, %q)",
				tt.in, gotMethod, gotPath, tt.wantMethod, tt.wantPath)
		}
	}
}

func TestBuildFullPath(t *testing.T) {
	// root -> /api -> /users, mirroring nested Route calls.
	root := &model.RouteNode{PathPrefix: "/"}
	api := &model.RouteNode{PathPrefix: "/api", Parent: root}
	users := &model.RouteNode{PathPrefix: "/users", Parent: api}
	bare := &model.RouteNode{Parent: root}

	tests := []struct {
		name string
		node *model.RouteNode
		path string
		want string
	}{
		{"nested", users, "/{id}", "/api/users/{id}"},
		{"nested index", users, "/", "/api/users"},
		{"one level", api, "/health", "/api/health"},
		{"no prefix", bare, "/ok", "/ok"},
		{"root index", bare, "/", "/"},
		{"path without leading slash", users, "x", "/api/users/x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildFullPath(tt.node, tt.path); got != tt.want {
				t.Errorf("buildFullPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestEndpointPatternFor(t *testing.T) {
	methodArg := 0
	def := &config.RouterDefinition{
		EndpointMethods: []string{"Get", "Post"},
		EndpointPatterns: []config.EndpointPattern{
			{Methods: []string{"Handle"}, MethodArg: &methodArg, PathArg: 1, HandlerArg: 2},
		},
	}

	// An explicit pattern wins and carries its own argument layout.
	got, ok := endpointPatternFor(def, "Handle")
	if !ok || got.PathArg != 1 || got.HandlerArg != 2 || got.MethodArg == nil {
		t.Errorf("Handle resolved to %+v, ok=%v", got, ok)
	}

	// A plain verb method falls back to (path, handler).
	got, ok = endpointPatternFor(def, "Get")
	if !ok || got.PathArg != 0 || got.HandlerArg != 1 || got.MethodArg != nil {
		t.Errorf("Get resolved to %+v, ok=%v", got, ok)
	}

	if _, ok := endpointPatternFor(def, "Use"); ok {
		t.Error("Use should not be treated as an endpoint method")
	}
}

// TestSplitMethodPatternNoVerb documents that a verb-less pattern is reported
// rather than fanned out across every method. Emitting five identical
// operations for one static file tree was worse than emitting none.
func TestSplitMethodPatternNoVerb(t *testing.T) {
	method, path := splitMethodPattern("/static/")
	if method != "" {
		t.Errorf("expected no method for %q, got %q", "/static/", method)
	}
	if path != "/static/" {
		t.Errorf("path = %q, want %q", path, "/static/")
	}
}
