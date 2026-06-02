// Package directpath implements simple kustomization directory detection
// based on changed files, without relying on ArgoCD ApplicationSets.
// This is suitable for simpler repo structures like internal-infra-deployments.
package directpath

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/aurelbalteaux/infra-tools/internal/appset"
	"github.com/aurelbalteaux/infra-tools/internal/detector"
)

// RepoQuerier matches the interface used by the detector package.
type RepoQuerier interface {
	Root() string
	DirExists(rel string) bool
	AbsPath(rel string) string
}

// Detector finds kustomization directories based on changed files.
type Detector struct {
	head           RepoQuerier
	componentsDir  string // e.g., "components"
	environmentMap map[string]detector.Environment
}

// NewDetector creates a Detector for direct path-based detection.
// componentsDir is the base directory to search (e.g., "components").
// environmentMap maps directory names to environments (e.g., "staging" -> Staging).
func NewDetector(head RepoQuerier, componentsDir string, environmentMap map[string]detector.Environment) *Detector {
	if environmentMap == nil {
		// Default environment mapping for internal-infra-deployments structure
		environmentMap = map[string]detector.Environment{
			"development": detector.Development,
			"staging":     detector.Staging,
			"production":  detector.Production,
		}
	}
	return &Detector{
		head:           head,
		componentsDir:  componentsDir,
		environmentMap: environmentMap,
	}
}

// AffectedComponents finds all kustomization directories that contain
// or are ancestors of the changed files.
func (d *Detector) AffectedComponents(changedFiles []string) (map[detector.Environment][]appset.ComponentPath, error) {
	result := make(map[detector.Environment][]appset.ComponentPath)
	seen := make(map[string]bool)

	for _, file := range changedFiles {
		// Only process files under the components directory
		if !strings.HasPrefix(file, d.componentsDir+"/") {
			continue
		}

		// Find the kustomization directory for this file
		kustomizationDir := d.findKustomizationDir(file)
		if kustomizationDir == "" {
			slog.Debug("no kustomization.yaml found for file", "file", file)
			continue
		}

		// Skip duplicates
		if seen[kustomizationDir] {
			continue
		}
		seen[kustomizationDir] = true

		// Determine environment from the path
		env := d.inferEnvironment(kustomizationDir)
		if env == "" {
			slog.Warn("cannot determine environment for path", "path", kustomizationDir)
			continue
		}

		// Extract cluster directory if present
		clusterDir := d.extractClusterDir(kustomizationDir, env)

		cp := appset.ComponentPath{
			Path:       kustomizationDir,
			ClusterDir: clusterDir,
		}

		result[env] = append(result[env], cp)
		slog.Info("Found affected component", "path", kustomizationDir, "env", env, "cluster", clusterDir)
	}

	return result, nil
}

// findKustomizationDir walks up the directory tree from the given file path
// until it finds a directory containing kustomization.yaml or kustomization.yml.
func (d *Detector) findKustomizationDir(filePath string) string {
	dir := filepath.Dir(filePath)

	for {
		// Check if this directory contains a kustomization file
		if d.head.DirExists(dir) {
			absDir := d.head.AbsPath(dir)
			if fileExists(filepath.Join(absDir, "kustomization.yaml")) ||
				fileExists(filepath.Join(absDir, "kustomization.yml")) {
				return dir
			}
		}

		// Stop if we've reached the components directory root
		if dir == d.componentsDir || dir == "." || dir == "/" {
			break
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached root
		}
		dir = parent
	}

	return ""
}

// inferEnvironment extracts the environment from the kustomization path.
// For example: components/monitoring/blackbox-exporter/staging/kustomization.yaml -> Staging
func (d *Detector) inferEnvironment(kustomizationPath string) detector.Environment {
	parts := strings.Split(kustomizationPath, string(filepath.Separator))

	// Look for environment names in the path
	for _, part := range parts {
		if env, ok := d.environmentMap[part]; ok {
			return env
		}
	}

	return ""
}

// extractClusterDir attempts to extract a cluster-specific directory from the path.
// For example: components/monitoring/blackbox-exporter/production/private/kflux-ocp-p01
// would return "kflux-ocp-p01".
func (d *Detector) extractClusterDir(kustomizationPath string, env detector.Environment) string {
	parts := strings.Split(kustomizationPath, string(filepath.Separator))

	// Find the environment index
	envIndex := -1
	for i, part := range parts {
		if envVal, ok := d.environmentMap[part]; ok && envVal == env {
			envIndex = i
			break
		}
	}

	if envIndex == -1 || envIndex == len(parts)-1 {
		return ""
	}

	// Look for cluster directory after the environment
	// Skip intermediate directories like "private", "public", "base"
	reservedDirs := map[string]bool{
		"base":    true,
		"private": true,
		"public":  true,
		"overlay": true,
	}

	for i := envIndex + 1; i < len(parts); i++ {
		if !reservedDirs[parts[i]] {
			return parts[i]
		}
	}

	return ""
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
