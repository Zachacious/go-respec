// Package testsuite holds end-to-end golden tests for the full respec
// pipeline: config load -> static analysis -> spec assembly -> YAML.
//
// Each subdirectory of testdata is a self-contained Go module (with a replace
// directive pointing back at this repo) representing a routing style we care
// about. Running the pipeline over it must produce want.yaml byte-for-byte.
//
// Regenerate goldens after an intentional behaviour change:
//
//	go test ./internal/testsuite -update
//
// Always read the resulting diff. These goldens are the only thing standing
// between a refactor of the analyzer and a silently wrong spec.
package testsuite

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zachacious/go-respec/internal/analyzer"
	"github.com/Zachacious/go-respec/internal/assembler"
	"github.com/Zachacious/go-respec/internal/config"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

var update = flag.Bool("update", false, "rewrite testdata/*/want.yaml from current behaviour")

// generate runs the complete pipeline against a fixture module and returns the
// marshalled spec, exactly as the CLI would write it.
func generate(t *testing.T, dir string) []byte {
	t.Helper()

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load(%s): %v", dir, err)
	}

	apiModel, err := analyzer.Analyze(dir, cfg)
	if err != nil {
		t.Fatalf("analyzer.Analyze(%s): %v", dir, err)
	}

	spec, err := assembler.BuildSpec(apiModel, cfg)
	if err != nil {
		t.Fatalf("assembler.BuildSpec(%s): %v", dir, err)
	}

	out, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatalf("yaml.Marshal(%s): %v", dir, err)
	}
	return out
}

func TestGolden(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join("testdata", entry.Name())
			golden := filepath.Join(dir, "want.yaml")
			got := generate(t, dir)

			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("updated %s", golden)
				return
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}

			if string(got) != string(want) {
				t.Errorf("spec mismatch for %s\n--- want ---\n%s\n--- got ---\n%s",
					entry.Name(), want, got)
			}
		})
	}
}

// TestGeneratedSpecsAreValid checks every fixture's output against OpenAPI 3.1.
//
// respec ships a validate command, so a spec respec emits that respec rejects is
// a contradiction users will hit immediately. This has regressed before, via
// security schemes that lost required fields and response headers with no schema.
func TestGeneratedSpecsAreValid(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()

			golden := filepath.Join("testdata", entry.Name(), "want.yaml")
			data, err := os.ReadFile(golden)
			if err != nil {
				t.Skipf("no golden yet: %v", err)
			}

			loader := openapi3.NewLoader()
			doc, err := loader.LoadFromData(data)
			if err != nil {
				t.Fatalf("loading generated spec failed: %v", err)
			}
			if err := doc.Validate(loader.Context); err != nil {
				t.Errorf("generated spec is not valid OpenAPI 3.1: %v", err)
			}
		})
	}
}

// TestSpecRoundTrips loads each generated spec and re-marshals it, requiring
// the bytes to come back identical.
//
// respec both writes specs and reads them back (`respec validate`), using the
// same library for each. This test is the tripwire for that library changing
// how it serialises: an upgrade that silently drops or reshapes a field shows
// up here as a diff rather than in a user's published documentation.
func TestSpecRoundTrips(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()

			golden := filepath.Join("testdata", entry.Name(), "want.yaml")
			original, err := os.ReadFile(golden)
			if err != nil {
				t.Skipf("no golden yet: %v", err)
			}

			loader := openapi3.NewLoader()
			doc, err := loader.LoadFromData(original)
			if err != nil {
				t.Fatalf("load: %v", err)
			}

			reMarshalled, err := yaml.Marshal(doc)
			if err != nil {
				t.Fatalf("re-marshal: %v", err)
			}

			if string(reMarshalled) != string(original) {
				t.Errorf("spec did not survive a load/marshal round trip\n--- original ---\n%s\n--- round-tripped ---\n%s",
					original, reMarshalled)
			}
		})
	}
}

// TestDeterministic guards against map-iteration order leaking into output.
// A flaky spec is worse than a wrong one, because it makes every diff noise.
func TestDeterministic(t *testing.T) {
	dir := filepath.Join("testdata", "chi_basic")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("chi_basic fixture not present")
	}
	first := generate(t, dir)
	for i := range 3 {
		if got := generate(t, dir); string(got) != string(first) {
			t.Fatalf("run %d differed from run 0:\n--- run 0 ---\n%s\n--- run %d ---\n%s",
				i+1, first, i+1, got)
		}
	}
}
