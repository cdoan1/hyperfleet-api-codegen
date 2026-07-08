package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cdoan1/hyperfleet-api-codegen/pkg/markers"
	"github.com/cdoan1/hyperfleet-api-codegen/pkg/passthrough"
)

func main() {
	var (
		sourceDir    string
		importPath   string
		outputDir    string
		typeNames    string
		registryFile string
		packageName  string
	)

	flag.StringVar(&sourceDir, "source-dir", "", "Directory containing source Go files (use this OR -import-path)")
	flag.StringVar(&importPath, "import-path", "", "Go import path to resolve via go.mod (use this OR -source-dir)")
	flag.StringVar(&outputDir, "output-dir", "", "Directory for generated output (required)")
	flag.StringVar(&typeNames, "types", "", "Comma-separated list of type names to generate (required)")
	flag.StringVar(&registryFile, "registry", "", "Path to field metadata registry (optional)")
	flag.StringVar(&packageName, "package", "v1alpha1", "Package name for generated code")
	flag.Parse()

	// Validate flags
	if outputDir == "" || typeNames == "" {
		flag.Usage()
		os.Exit(1)
	}

	if sourceDir == "" && importPath == "" {
		log.Fatalf("Either -source-dir or -import-path must be specified")
	}

	if sourceDir != "" && importPath != "" {
		log.Fatalf("Cannot specify both -source-dir and -import-path")
	}

	// Parse type names
	types := strings.Split(typeNames, ",")
	for i := range types {
		types[i] = strings.TrimSpace(types[i])
	}

	// Load registry if provided
	registry := make(markers.FieldRegistry)
	if registryFile != "" {
		log.Printf("Loading field registry from: %s", registryFile)
		// TODO: Implement registry loading from file
		// For now, just use empty registry
	}

	// Create generator
	var gen *passthrough.Generator
	var err error

	if importPath != "" {
		log.Printf("Resolving import path: %s", importPath)
		gen, err = passthrough.NewGeneratorFromImportPath(importPath, types, registry)
		if err != nil {
			log.Fatalf("Failed to resolve import path: %v", err)
		}
		log.Printf("Resolved to directory: %s", gen.SourceDir)
	} else {
		gen = passthrough.NewGenerator(sourceDir, types, registry)
	}

	gen.OutputPackage = packageName

	// Load source files
	log.Printf("Loading source files from: %s", gen.SourceDir)
	if err := gen.LoadSourceFiles(gen.SourceDir); err != nil {
		log.Fatalf("Failed to load source files: %v", err)
	}

	log.Printf("Loaded %d source files", len(gen.ParsedFiles()))

	// Generate passthrough types
	log.Printf("Generating passthrough types: %v", types)
	if err := gen.Generate(outputDir); err != nil {
		log.Fatalf("Failed to generate: %v", err)
	}

	fmt.Printf("Successfully generated passthrough types in %s\n", outputDir)
}
