package kustomize

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// OrphanedFile represents a YAML file that is not referenced in any kustomization.yaml
type OrphanedFile struct {
	Path         string // Relative path to the file
	KustomizeDir string // Directory containing kustomization.yaml
	AbsolutePath string // Absolute path to the file
}

// ValidationResult contains the results of validating kustomization files
type ValidationResult struct {
	OrphanedFiles []OrphanedFile
	CheckedDirs   int
}

// ValidateAllReferences walks a directory tree and checks that every YAML file
// is referenced in its parent kustomization.yaml file.
func ValidateAllReferences(rootDir string) (*ValidationResult, error) {
	result := &ValidationResult{
		OrphanedFiles: []OrphanedFile{},
	}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-directories
		if !info.IsDir() {
			return nil
		}

		// Check if this directory contains a kustomization file
		kustomizationPath := filepath.Join(path, "kustomization.yaml")
		if !fileExists(kustomizationPath) {
			kustomizationPath = filepath.Join(path, "kustomization.yml")
			if !fileExists(kustomizationPath) {
				return nil // No kustomization in this directory
			}
		}

		// Found a kustomization directory - validate it
		orphaned, err := validateDirectory(rootDir, path)
		if err != nil {
			return fmt.Errorf("validating %s: %w", path, err)
		}

		result.OrphanedFiles = append(result.OrphanedFiles, orphaned...)
		result.CheckedDirs++

		return nil
	})

	return result, err
}

// validateDirectory checks a single directory with a kustomization.yaml file
func validateDirectory(rootDir, dir string) ([]OrphanedFile, error) {
	// Load the kustomization file
	fSys := filesys.MakeFsOnDisk()
	kustomization, err := loadKustomization(dir, fSys)
	if err != nil {
		return nil, err
	}

	// Build a set of all referenced files
	referenced := make(map[string]bool)

	// Add all resources
	for _, resource := range kustomization.Resources {
		// Normalize the path
		normalized := normalizeResourcePath(resource)
		referenced[normalized] = true
	}

	// Add all patches
	for _, patch := range kustomization.Patches {
		if patch.Path != "" {
			referenced[normalizeResourcePath(patch.Path)] = true
		}
	}

	// Add deprecated patches
	//nolint:staticcheck // SA1019: Intentionally support deprecated kustomization fields for backward compatibility
	for _, patch := range kustomization.PatchesStrategicMerge {
		referenced[normalizeResourcePath(string(patch))] = true
	}

	//nolint:staticcheck // SA1019: Intentionally support deprecated kustomization fields for backward compatibility
	for _, patch := range kustomization.PatchesJson6902 {
		if patch.Path != "" {
			referenced[normalizeResourcePath(patch.Path)] = true
		}
	}

	// Add components
	for _, component := range kustomization.Components {
		referenced[normalizeResourcePath(component)] = true
	}

	// Bases (deprecated, but still used)
	//nolint:staticcheck // SA1019: Intentionally support deprecated kustomization fields for backward compatibility
	for _, base := range kustomization.Bases {
		referenced[normalizeResourcePath(base)] = true
	}

	// CRDs
	for _, crd := range kustomization.Crds {
		referenced[normalizeResourcePath(crd)] = true
	}

	// Configurations
	for _, config := range kustomization.Configurations {
		referenced[normalizeResourcePath(config)] = true
	}

	// Find all YAML files in the directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var orphaned []OrphanedFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip kustomization files themselves
		if name == "kustomization.yaml" || name == "kustomization.yml" {
			continue
		}

		// Only check YAML files
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		// Check if this file is referenced
		if !referenced[name] {
			relPath, err := filepath.Rel(rootDir, filepath.Join(dir, name))
			if err != nil {
				relPath = filepath.Join(dir, name)
			}
			relDir, err := filepath.Rel(rootDir, dir)
			if err != nil {
				relDir = dir
			}
			orphaned = append(orphaned, OrphanedFile{
				Path:         relPath,
				KustomizeDir: relDir,
				AbsolutePath: filepath.Join(dir, name),
			})
		}
	}

	return orphaned, nil
}

// loadKustomization loads a kustomization file from a directory
func loadKustomization(dir string, fSys filesys.FileSystem) (*types.Kustomization, error) {
	// Try kustomization.yaml first
	content, err := fSys.ReadFile(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		// Try kustomization.yml
		content, err = fSys.ReadFile(filepath.Join(dir, "kustomization.yml"))
		if err != nil {
			return nil, fmt.Errorf("reading kustomization file: %w", err)
		}
	}

	kustomization := &types.Kustomization{}
	err = kustomization.Unmarshal(content)
	if err != nil {
		return nil, fmt.Errorf("parsing kustomization file: %w", err)
	}

	return kustomization, nil
}

// normalizeResourcePath normalizes a resource path for comparison.
// Removes directory references and keeps just the filename for local files.
func normalizeResourcePath(path string) string {
	// If it's a URL or external reference, keep it as is
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") ||
		strings.Contains(path, "?ref=") || strings.Contains(path, "?version=") {
		return path
	}

	// Directory references (parent/relative/subdir) should not match local files
	// Keep full path to avoid false matches with local basenames
	if strings.Contains(path, "/") {
		return path
	}

	// Local files in same directory - no change needed
	return path
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
