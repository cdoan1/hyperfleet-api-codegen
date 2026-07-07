package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/openshift-online/hyperfleet-api-codegen/pkg/markers"
	"github.com/openshift-online/hyperfleet-api-codegen/pkg/passthrough"
)

func main() {
	var (
		sourceDir    string
		outputDir    string
		typeNames    string
		registryFile string
		packageName  string
	)

	flag.StringVar(&sourceDir, "source-dir", "", "Directory containing source Go files (required)")
	flag.StringVar(&outputDir, "output-dir", "", "Directory for generated output (required)")
	flag.StringVar(&typeNames, "types", "", "Comma-separated list of type names to generate (required)")
	flag.StringVar(&registryFile, "registry", "", "Path to field metadata registry (optional)")
	flag.StringVar(&packageName, "package", "v1alpha1", "Package name for generated code")
	flag.Parse()

	if sourceDir == "" || outputDir == "" || typeNames == "" {
		flag.Usage()
		os.Exit(1)
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
	gen := passthrough.NewGenerator(sourceDir, types, registry)
	gen.OutputPackage = packageName

	// Load source files
	log.Printf("Loading source files from: %s", sourceDir)
	if err := gen.LoadSourceFiles(sourceDir); err != nil {
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
