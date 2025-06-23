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
	// Create a packages configuration with the required load modes.
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

	// --- Perform analysis phases ---
	fmt.Println("Phase 1: Discovering universe...")
	state.discoverUniverse()

	fmt.Println("Phase 2: Parsing handler metadata...")
	state.FindAndParseRouteMetadata() // <-- ADDED THIS MISSING STEP

	fmt.Println("Phase 3: Performing data flow analysis...")
	state.performDataFlowAnalysis()

	fmt.Println("Phase 4: Finding group metadata...")
	state.FindGroupMetadata()

	fmt.Println("Phase 5: Analyzing handler bodies...")
	state.analyzeHandlers()

	fmt.Println("✅ Analysis complete. All phases executed successfully.")

	// ... (API model assembly is unchanged) ...
	apiModel := &model.APIModel{}
	apiModel.RouteGraph = state.RouteGraph
	apiModel.GroupMetadata = state.GroupMetadata
	if apiModel.Components == nil {
		apiModel.Components = &openapi3.Components{}
	}
	if apiModel.Components.Schemas == nil {
		apiModel.Components.Schemas = make(map[string]*openapi3.SchemaRef)
	}
	for _, ref := range state.SchemaGen.schemas {
		key := strings.TrimPrefix(ref.Ref, "#/components/schemas/")
		apiModel.Components.Schemas[key] = ref
	}

	return apiModel, nil
}
