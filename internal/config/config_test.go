package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name  string
		style string
		in    string
		want  string
	}{
		{"brace passes through", PathParamBrace, "/users/{id}", "/users/{id}"},
		{"colon becomes brace", PathParamColon, "/users/:id", "/users/{id}"},
		{"multiple colon params", PathParamColon, "/users/:id/posts/:postID", "/users/{id}/posts/{postID}"},
		{"colon mid-path", PathParamColon, "/users/:id/posts", "/users/{id}/posts"},
		{"no params", PathParamColon, "/users", "/users"},
		{"empty style defaults to brace", "", "/users/{id}", "/users/{id}"},
		// A colon-style router must not mangle a literal that merely contains a
		// colon, such as a port in a mounted host pattern.
		{"brace style leaves colons alone", PathParamBrace, "/proxy/:8080", "/proxy/:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &RouterDefinition{PathParamStyle: tt.style}
			if got := def.NormalizePath(tt.in); got != tt.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestPathParamNames(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"/users/{id}", []string{"id"}},
		{"/users/{id}/posts/{postID}", []string{"id", "postID"}},
		{"/users", nil},
		{"/", nil},
	}

	for _, tt := range tests {
		got := PathParamNames(tt.in)
		if len(got) == 0 && len(tt.want) == 0 {
			continue
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("PathParamNames(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// TestSecuritySchemeRoundTrip guards the fields that were previously dropped.
// An apiKey scheme without `in` and `name` produces a spec that respec's own
// validate command rejects.
func TestSecuritySchemeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	yaml := `
info:
  title: "T"
  version: "1"
securitySchemes:
  ApiKeyAuth:
    type: apiKey
    in: header
    name: X-API-Key
  BearerAuth:
    type: http
    scheme: bearer
    bearerFormat: JWT
  OIDC:
    type: openIdConnect
    openIdConnectUrl: https://example.com/.well-known/openid-configuration
  OAuth:
    type: oauth2
    flows:
      authorizationCode:
        authorizationUrl: https://example.com/auth
        tokenUrl: https://example.com/token
        scopes:
          read: Read access
`
	if err := os.WriteFile(filepath.Join(dir, ".respec.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	schemes := cfg.GetSecuritySchemes()

	apiKey := schemes["ApiKeyAuth"]
	if apiKey == nil || apiKey.Value == nil {
		t.Fatal("ApiKeyAuth missing")
	}
	if apiKey.Value.In != "header" {
		t.Errorf("ApiKeyAuth.In = %q, want header", apiKey.Value.In)
	}
	if apiKey.Value.Name != "X-API-Key" {
		t.Errorf("ApiKeyAuth.Name = %q, want X-API-Key", apiKey.Value.Name)
	}

	bearer := schemes["BearerAuth"]
	if bearer == nil || bearer.Value.BearerFormat != "JWT" {
		t.Error("BearerAuth lost bearerFormat")
	}

	oidc := schemes["OIDC"]
	if oidc == nil || oidc.Value.OpenIdConnectUrl == "" {
		t.Error("OIDC lost openIdConnectUrl")
	}

	oauth := schemes["OAuth"]
	if oauth == nil || oauth.Value.Flows == nil || oauth.Value.Flows.AuthorizationCode == nil {
		t.Fatal("OAuth lost its flows")
	}
	if got := oauth.Value.Flows.AuthorizationCode.Scopes["read"]; got != "Read access" {
		t.Errorf("OAuth scope = %q, want %q", got, "Read access")
	}
}

// TestDefaultsAreClean makes sure no private paths leak back into the shipped
// configuration. The defaults previously referenced the author's own project.
func TestDefaultsAreClean(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	check := func(what, path string) {
		t.Helper()
		for _, bad := range []string{"justauth", "zachacious/"} {
			if len(path) >= len(bad) && contains(path, bad) {
				t.Errorf("%s references a private path: %q", what, path)
			}
		}
	}

	for _, p := range cfg.HandlerPatterns.RequestBody {
		check("requestBody", p.FunctionPath)
	}
	for _, p := range cfg.HandlerPatterns.ResponseBody {
		check("responseBody", p.FunctionPath)
	}
	for _, p := range cfg.SecurityPatterns {
		check("securityPatterns", p.FunctionPath)
	}

	// encoding/json.Encoder.Encode must not be listed: the analyzer handles it
	// directly so that WriteHeader sets the status. A config entry as well
	// would register a second, always-200 response for the same call.
	for _, p := range cfg.HandlerPatterns.ResponseBody {
		if p.FunctionPath == "encoding/json.Encoder.Encode" {
			t.Error("encoding/json.Encoder.Encode must not be a default responseBody pattern")
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
