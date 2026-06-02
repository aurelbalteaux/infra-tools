// Command validate-refs checks that all YAML files in a directory tree are
// referenced in their parent kustomization.yaml files.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aurelbalteaux/infra-tools/internal/ci"
	ghclient "github.com/aurelbalteaux/infra-tools/internal/github"
	"github.com/aurelbalteaux/infra-tools/internal/kustomize"
	"github.com/aurelbalteaux/infra-tools/internal/logging"
)

// version is set via -ldflags at build time.
var version = "dev"

func main() {
	var (
		rootDir     = flag.String("root", "", "Root directory to validate (required)")
		showVersion = flag.Bool("version", false, "Print version and exit")
		verbose     = flag.Bool("verbose", false, "Show all checked directories")
		outputMode  = flag.String("output-mode", "local", "Output mode: local (stdout), ci-comment (GitHub PR comment)")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("validate-refs %s\n", version)
		os.Exit(0)
	}

	if *rootDir == "" {
		fmt.Fprintf(os.Stderr, "Error: --root is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Validate output mode
	switch *outputMode {
	case "local", "ci-comment":
		// valid
	default:
		fmt.Fprintf(os.Stderr, "invalid --output-mode %q: must be one of local, ci-comment\n", *outputMode)
		os.Exit(1)
	}

	// Set up logging for ci-comment mode
	if *outputMode == "ci-comment" {
		logCleanup, err := logging.Setup("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to set up logging: %v\n", err)
			os.Exit(1)
		}
		if logCleanup != nil {
			defer logCleanup()
		}
	}

	absRoot, err := filepath.Abs(*rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving root directory: %v\n", err)
		os.Exit(1)
	}

	// Check that the root directory exists
	if info, err := os.Stat(absRoot); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a valid directory\n", absRoot)
		os.Exit(1)
	}

	if *outputMode == "local" {
		fmt.Printf("Validating YAML references in: %s\n\n", absRoot)
	}

	result, err := kustomize.ValidateAllReferences(absRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during validation: %v\n", err)
		os.Exit(1)
	}

	// Handle output based on mode
	switch *outputMode {
	case "local":
		// Original stdout output
		if *verbose {
			fmt.Printf("Checked %d directories with kustomization files\n\n", result.CheckedDirs)
		}

		if len(result.OrphanedFiles) == 0 {
			fmt.Println("✓ All YAML files are properly referenced in kustomization files")
			return
		}

		// Group orphaned files by directory for better readability
		byDir := make(map[string][]kustomize.OrphanedFile)
		for _, orphan := range result.OrphanedFiles {
			byDir[orphan.KustomizeDir] = append(byDir[orphan.KustomizeDir], orphan)
		}

		fmt.Printf("✗ Found %d orphaned YAML file(s):\n\n", len(result.OrphanedFiles))

		for _, orphans := range byDir {
			// Use the relative path from the first orphan in this directory
			if len(orphans) > 0 {
				relDir := filepath.Dir(orphans[0].Path)
				fmt.Printf("  Directory: %s/\n", relDir)
				for _, orphan := range orphans {
					fmt.Printf("    - %s\n", filepath.Base(orphan.Path))
				}
				fmt.Println()
			}
		}

		fmt.Printf("These files should be added to their respective kustomization.yaml files\n")
		fmt.Printf("or removed if they are no longer needed.\n")
		os.Exit(1)

	case "ci-comment":
		ctx := context.Background()
		if err := postCIComment(ctx, result); err != nil {
			fmt.Fprintf(os.Stderr, "Error posting PR comment: %v\n", err)
			os.Exit(1)
		}
		// Exit with error code if validation failed
		if len(result.OrphanedFiles) > 0 {
			os.Exit(1)
		}
	}
}

// buildCommentBody creates a markdown PR comment from validation results.
func buildCommentBody(result *kustomize.ValidationResult, runURL string) string {
	var b strings.Builder

	fmt.Fprintln(&b, "<!-- validate-refs-comment -->")
	fmt.Fprintln(&b, "### YAML Reference Validation")
	fmt.Fprintln(&b)

	if len(result.OrphanedFiles) == 0 {
		fmt.Fprintln(&b, "✅ All YAML files are properly referenced in kustomization files")
		fmt.Fprintf(&b, "\n_Checked %d directories_\n", result.CheckedDirs)
		return b.String()
	}

	// Group orphaned files by directory
	byDir := make(map[string][]kustomize.OrphanedFile)
	for _, orphan := range result.OrphanedFiles {
		byDir[orphan.KustomizeDir] = append(byDir[orphan.KustomizeDir], orphan)
	}

	fmt.Fprintf(&b, "❌ Found **%d orphaned YAML file(s)** that should be added to kustomization.yaml files:\n\n", len(result.OrphanedFiles))

	for dir, orphans := range byDir {
		fmt.Fprintf(&b, "**Directory:** `%s`\n", dir)
		for _, orphan := range orphans {
			fmt.Fprintf(&b, "- `%s`\n", filepath.Base(orphan.Path))
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "---")
	fmt.Fprintln(&b, "_These files should be added to their respective kustomization.yaml files or removed if no longer needed._")

	if runURL != "" {
		fmt.Fprintf(&b, "\n📋 [View workflow run](%s)\n", runURL)
	}

	return b.String()
}

// postCIComment posts validation results as a PR comment using the shared ci package.
func postCIComment(ctx context.Context, result *kustomize.ValidationResult) error {
	return ci.PostPRComment(ctx, ghclient.ValidateRefsCommentMarker, func(runURL string) string {
		return buildCommentBody(result, runURL)
	})
}
