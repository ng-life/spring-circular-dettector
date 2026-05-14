package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"spring-circular-detector/checker"
	"spring-circular-detector/report"
)

func main() {
	path := flag.String("path", "", "Path to Java project source directory")
	verbose := flag.Bool("v", false, "Verbose: print parsed bean information")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -path <java-project-src-dir> [-v]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nSpring Circular Dependency & Dubbo @Reference Checker\n")
		fmt.Fprintf(os.Stderr, "\nChecks:\n")
		fmt.Fprintf(os.Stderr, "  1. Circular dependencies (including @Reference chains)\n")
		fmt.Fprintf(os.Stderr, "  2. Improper @Reference usage (referencing local Dubbo services)\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *path == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Verify path exists
	info, err := os.Stat(*path)
	if err != nil {
		log.Fatalf("Path does not exist: %s", *path)
	}
	if !info.IsDir() {
		log.Fatalf("Path is not a directory: %s", *path)
	}

	fmt.Printf("Analyzing: %s\n\n", *path)

	// Check circular dependencies
	fmt.Println("Checking circular dependencies...")
	cycles, err := checker.CheckCircular(*path)
	if err != nil {
		log.Fatalf("Error checking circular dependencies: %v", err)
	}

	// Check improper @Reference usage
	fmt.Println("Checking @Reference usage...")
	refIssues, err := checker.CheckDubboReference(*path)
	if err != nil {
		log.Fatalf("Error checking @Reference usage: %v", err)
	}

	if *verbose {
		// Re-parse for verbose output (simple approach)
		fmt.Println("\n=== Bean Summary ===")
	}

	report.PrintResults(cycles, refIssues)

	if len(cycles) == 0 && len(refIssues) == 0 {
		os.Exit(0)
	}
	os.Exit(1)
}
