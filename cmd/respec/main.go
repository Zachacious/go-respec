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

// These variables are set at build time by the Makefile's ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var outputPath string

	var rootCmd = &cobra.Command{
		Use:   "respec [path]",
		Short: "respec is a Go static analysis tool to generate OpenAPI specs without magic comments.",
		Long: `respec analyzes a Go project's source code to infer API routes, handlers,
and schemas, producing a valid OpenAPI v3 specification. It is designed to be
framework-agnostic and highly configurable through a .respec.yaml file.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			projectPath := args[0]
			fmt.Printf("Starting analysis of project at: %s\n", projectPath)
			cfg, err := config.Load(projectPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading .respec.yaml: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Configuration loaded.")
			a, err := analyzer.New(projectPath, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error initializing analyzer: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Analyzer initialized.")
			apiModel, err := a.Analyze()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error during analysis: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Analysis complete. Assembling specification...")
			spec, err := assembler.BuildSpec(apiModel, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error assembling specification: %v\n", err)
				os.Exit(1)
			}
			yamlData, err := yaml.Marshal(spec)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling spec to YAML: %v\n", err)
				os.Exit(1)
			}
			err = os.WriteFile(outputPath, yamlData, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Successfully generated OpenAPI spec at: %s\n", outputPath)
		},
	}

	// Add the new `version` command to the root command.
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of respec",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("respec version %s\n", version)
			fmt.Printf("commit: %s\n", commit)
			fmt.Printf("built at: %s\n", date)
		},
	}
	rootCmd.AddCommand(versionCmd)

	rootCmd.Flags().StringVarP(&outputPath, "output", "o", "openapi.yaml", "Output file for the OpenAPI specification")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
