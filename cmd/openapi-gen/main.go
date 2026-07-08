package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/cdoan1/hyperfleet-api-codegen/pkg/openapi"
)

func main() {
	var (
		outputFile string
		title      string
		version    string
	)

	flag.StringVar(&outputFile, "output-file", "", "Output file for OpenAPI schema (required)")
	flag.StringVar(&title, "title", "HyperFleet API", "API title")
	flag.StringVar(&version, "version", "v1alpha1", "API version")
	flag.Parse()

	if outputFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	// For this POC, we're not scanning directories yet
	// In a full implementation, we'd pass input directories
	gen := openapi.NewGenerator(nil, outputFile)
	gen.Title = title
	gen.Version = version

	log.Printf("Generating OpenAPI schema: %s v%s", title, version)
	if err := gen.Generate(); err != nil {
		log.Fatalf("Failed to generate OpenAPI schema: %v", err)
	}

	fmt.Printf("Successfully generated OpenAPI schema at %s\n", outputFile)
	fmt.Println("\nNote: This is a POC implementation.")
	fmt.Println("Full implementation would use openapi-gen to scan Go types and respect +k8s:openapi-gen markers.")
}
