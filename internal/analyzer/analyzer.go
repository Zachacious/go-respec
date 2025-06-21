package analyzer

import (
	"fmt"
	"strings"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
	"golang.org/x/tools/go/packages"
)

// Analyze is the main entry point that orchestrates the entire analysis pipeline.
func Analyze(projectPath string, cfg *config.Config) (*model.APIModel, error) {
	pkgCfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
		Dir: projectPath,
	}
	pkgs, err := packages.Load(pkgCfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("failed to load packages: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("packages contain errors")
	}

	state, err := NewState(pkgs, cfg)
	if err != nil {
		return nil, err
	}
	state.discoverUniverse()
	state.performDataFlowAnalysis()
	state.analyzeHandlers()

	fmt.Println("✅ Analysis complete. All phases executed successfully.")

	apiModel := &model.APIModel{}
	apiModel.RouteGraph = state.RouteGraph

	if apiModel.Components == nil {
		apiModel.Components = &openapi3.Components{}
	}
	// --- Start of definitive fix ---
	// Use the explicit map type to avoid any ambiguity with the type alias.
	if apiModel.Components.Schemas == nil {
		apiModel.Components.Schemas = make(map[string]*openapi3.SchemaRef)
	}
	// --- End of definitive fix ---

	for _, ref := range state.SchemaGen.schemas {
		key := strings.TrimPrefix(ref.Ref, "#/components/schemas/")
		apiModel.Components.Schemas[key] = ref
	}

	return apiModel, nil
}
