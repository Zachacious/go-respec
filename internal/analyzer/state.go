package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/Zachacious/go-respec/internal/config"
	"github.com/Zachacious/go-respec/internal/model"
	"golang.org/x/tools/go/packages"
)

// TrackedValue represents a value (e.g., a router instance) that we are
// tracing through the program's data flow.
type TrackedValue struct {
	// The expression where this value was created (e.g., the `chi.NewRouter()` call).
	Source ast.Expr
	// The specific router definition this value corresponds to.
	RouterDef *config.RouterDefinition

	// --- Chaining and Hierarchy ---
	Parent     *TrackedValue
	PathPrefix string
	// The node in our API graph that corresponds to this specific router or group.
	Node *model.RouteNode
}

// WorklistItem represents a single task for our data flow analysis.
// It contains a node to analyze and the value associated with it.
type WorklistItem struct {
	// The AST node representing the usage of a tracked value (e.g., an `*ast.Ident` for a variable).
	Node ast.Node
	// The value being tracked at this node.
	Value *TrackedValue
}

// Universe contains maps of all relevant top-level declarations in the project.
// This serves as a quick lookup table for the rest of the analysis.
type Universe struct {
	// A map of function objects to their AST declaration nodes.
	Functions map[types.Object]*ast.FuncDecl

	// A map of constant objects to their value specifications.
	// This helps in resolving path segments that are defined as constants.
	Constants map[types.Object]*ast.ValueSpec
}

// ResolvedType represents a type from the config that has been resolved
// to its canonical go/types object.
type ResolvedType struct {
	// The canonical representation of the type. This is what we use for reliable comparisons.
	Object *types.Named
	// A pointer to the original definition from the config file.
	Definition *config.RouterDefinition
}

// State is the central data structure that holds all information
// gathered during the multi-phase analysis of the target project.
type State struct {
	// The initial loaded packages for the entire project.
	pkgs []*packages.Package

	// A map to quickly get the types.Info for a given AST file.
	fileTypeInfo map[*ast.File]*types.Info

	// A map of fully-qualified type names to their resolved canonical types.
	// This is populated by the resolver (Phase 1).
	// Example key: "github.com/go-chi/chi/v5.Mux"
	ResolvedRouterTypes map[string]*ResolvedType

	// The discovered universe of all relevant declarations in the project.
	// This is populated by the universe discoverer (Phase 2).
	Universe *Universe

	// --- Data Flow Analysis State ---
	// The queue of analysis tasks.
	Worklist []WorklistItem
	// A map to link expressions (like call results) to the tracked value they produce.
	ExprResults map[ast.Expr]*TrackedValue
	// A map to link variable/parameter objects to the tracked value they hold.
	VarValues map[types.Object]*TrackedValue
	// A map to prevent processing the same node for the same value twice, avoiding cycles.
	processed map[ast.Node]bool

	// The root of the final constructed API route graph.
	RouteGraph *model.RouteNode

	// --- Schema Generation State ---
	// The schema generator instance.
	SchemaGen *SchemaGenerator

	Config *config.Config

	Metadata MetadataMap
}

func NewState(pkgs []*packages.Package, cfg *config.Config) (*State, error) {
	fileInfoMap := make(map[*ast.File]*types.Info)
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			fileInfoMap[file] = pkg.TypesInfo
		}
	}

	s := &State{
		pkgs:                pkgs,
		fileTypeInfo:        fileInfoMap,
		ResolvedRouterTypes: make(map[string]*ResolvedType),
		Universe: &Universe{ // Initialize the universe
			Functions: make(map[types.Object]*ast.FuncDecl),
			Constants: make(map[types.Object]*ast.ValueSpec),
		},
		// Initialize flow analysis fields
		Worklist:    make([]WorklistItem, 0),
		ExprResults: make(map[ast.Expr]*TrackedValue),
		VarValues:   make(map[types.Object]*TrackedValue),
		processed:   make(map[ast.Node]bool),
		RouteGraph:  &model.RouteNode{PathPrefix: "/"},
		SchemaGen:   NewSchemaGenerator(),
		Config:      cfg,
		Metadata:    make(MetadataMap),
	}

	// Immediately run the resolver to populate our type map.
	if err := s.resolveConfigTypes(cfg); err != nil {
		return nil, fmt.Errorf("failed to resolve types from config: %w", err)
	}

	return s, nil
}
