package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/openshift-online/hyperfleet-api-codegen/pkg/markers"
)

func main() {
	var (
		inputDirs  string
		outputFile string
		validate   bool
	)

	flag.StringVar(&inputDirs, "input-dirs", "", "Comma-separated list of directories to scan (required)")
	flag.StringVar(&outputFile, "output-file", "", "Output file for generated registry (required)")
	flag.BoolVar(&validate, "validate", true, "Validate that all visible fields have write-mode markers")
	flag.Parse()

	if inputDirs == "" || outputFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	dirs := strings.Split(inputDirs, ",")
	for i := range dirs {
		dirs[i] = strings.TrimSpace(dirs[i])
	}

	// Create scanner and scan directories
	scanner := markers.NewScanner(dirs)

	log.Printf("Scanning directories: %v", dirs)
	if err := scanner.Scan(); err != nil {
		log.Fatalf("Error scanning: %v", err)
	}

	log.Printf("Found %d fields with markers", len(scanner.Registry))

	// Validate if requested
	if validate {
		if err := scanner.Registry.Validate(); err != nil {
			log.Fatalf("Validation failed: %v", err)
		}
		log.Println("Validation passed")
	}

	// Generate registry file
	log.Printf("Generating registry: %s", outputFile)
	if err := scanner.Generate(outputFile); err != nil {
		log.Fatalf("Error generating registry: %v", err)
	}

	fmt.Printf("Successfully generated field registry at %s\n", outputFile)
}
