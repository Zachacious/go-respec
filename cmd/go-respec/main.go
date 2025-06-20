package main

import (
	"fmt"
	"os"

	"github.com/Zachacious/go-respec/internal/analyzer"
	"github.com/Zachacious/go-respec/internal/assembler"
	"github.com/Zachacious/go-respec/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func main() {
	var outputPath string

	// rootCmd defines the base command when called without any subcommands.
	var rootCmd = &cobra.Command{
		Use:   "respec [path]",
		Short: "respec is a Go static analysis tool to generate OpenAPI specs without magic comments.",
		Long: `respec analyzes a Go project's source code to infer API routes, handlers,
and schemas, producing a valid OpenAPI v3 specification. It is designed to be
framework-agnostic and highly configurable through a .respec.yaml file.`,
		// We expect exactly one argument: the path to the project.
		Args: cobra.ExactArgs(1),
		// This is the main execution logic for the CLI tool.
		Run: func(cmd *cobra.Command, args []string) {
			projectPath := args[0]

			fmt.Printf("Starting analysis of project at: %s\n", projectPath)

			// 1. Load configuration from .respec.yaml at the project root.
			cfg, err := config.Load(projectPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading .respec.yaml: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Configuration loaded.")

			// 2. Initialize the analyzer with the project path and the loaded config.
			a, err := analyzer.New(projectPath, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error initializing analyzer: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Analyzer initialized.")

			// 3. Run the analysis to build the internal API model.
			apiModel, err := a.Analyze()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error during analysis: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Analysis complete. Assembling specification...")

			// 4. Assemble the final OpenAPI spec from the internal model and config.
			spec, err := assembler.BuildSpec(apiModel, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error assembling specification: %v\n", err)
				os.Exit(1)
			}

			// 5. Marshal the completed spec object into YAML format.
			yamlData, err := yaml.Marshal(spec)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling spec to YAML: %v\n", err)
				os.Exit(1)
			}

			// 6. Write the final YAML to the specified output file.
			err = os.WriteFile(outputPath, yamlData, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Successfully generated OpenAPI spec at: %s\n", outputPath)
		},
	}

	// Define the command-line flags.
	// Here we define the persistent "-o" or "--output" flag to specify the output file path.
	rootCmd.Flags().StringVarP(&outputPath, "output", "o", "openapi.yaml", "Output file for the OpenAPI specification")

	// Execute the root command.
	if err := rootCmd.Execute(); err != nil {
		// Cobra prints its own errors, so we just need to exit.
		os.Exit(1)
	}
}
