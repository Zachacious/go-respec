package analyzer

import (
	"fmt"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
	"golang.org/x/tools/go/packages"
)

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
	state.FindAndParseRouteMetadata() // Parse .Handler() calls
	state.FindGroupMetadata()         // Parse .Meta() calls
	state.performDataFlowAnalysis()
	state.analyzeHandlers()

	fmt.Println("✅ Analysis complete. All phases executed successfully.")

	apiModel := &model.APIModel{}
	apiModel.RouteGraph = state.RouteGraph
	apiModel.GroupMetadata = state.GroupMetadata

	if apiModel.Components == nil {
		apiModel.Components = &openapi3.Components{}
	}

	// This correctly uses the public Components map from the SchemaGenerator.
	// This ensures that only named, reusable schemas are added to the final spec.
	apiModel.Components.Schemas = state.SchemaGen.Components

	return apiModel, nil
}
