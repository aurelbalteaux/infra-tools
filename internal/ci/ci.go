// Package ci provides shared utilities for GitHub Actions CI workflows.
package ci

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	ghclient "github.com/aurelbalteaux/infra-tools/internal/github"
)

// BuildRunURL constructs a direct link to the current GitHub Actions workflow run
// using environment variables set by GitHub Actions. Returns empty string if any
// required environment variable is missing.
//
// Required environment variables:
//   - GITHUB_SERVER_URL: GitHub server URL (e.g., https://github.com)
//   - GITHUB_REPOSITORY: repository in "owner/repo" format
//   - GITHUB_RUN_ID: current workflow run ID
func BuildRunURL() string {
	serverURL := os.Getenv("GITHUB_SERVER_URL")
	repo := os.Getenv("GITHUB_REPOSITORY")
	runID := os.Getenv("GITHUB_RUN_ID")

	if serverURL == "" || repo == "" || runID == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/actions/runs/%s", serverURL, repo, runID)
}

// PostPRComment posts or updates a comment on a GitHub pull request using
// environment variables for configuration. The comment is identified by the
// provided marker for idempotent updates.
//
// Required environment variables:
//   - GITHUB_TOKEN: GitHub API token for authentication
//   - GITHUB_REPOSITORY: repository in "owner/repo" format
//   - PR_NUMBER: pull request number to comment on
//
// The buildBody callback is invoked with the workflow run URL (which may be empty)
// and should return the complete comment body markdown, including the comment marker.
//
// If any required environment variable is missing, the comment body is printed to
// stdout as a fallback instead of posting to GitHub.
func PostPRComment(ctx context.Context, marker string, buildBody func(runURL string) string) error {
	runURL := BuildRunURL()
	body := buildBody(runURL)

	token := os.Getenv("GITHUB_TOKEN")
	repo := os.Getenv("GITHUB_REPOSITORY")
	prStr := os.Getenv("PR_NUMBER")

	if token == "" || repo == "" || prStr == "" {
		// Missing CI env vars — print to stdout as fallback
		fmt.Print(body)
		return nil
	}

	prNumber := 0
	if _, err := fmt.Sscanf(prStr, "%d", &prNumber); err != nil || prNumber <= 0 {
		return fmt.Errorf("invalid PR_NUMBER %q", prStr)
	}

	client, err := ghclient.NewCommentClient(token, repo, marker)
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	if err := client.UpsertComment(ctx, prNumber, body); err != nil {
		return fmt.Errorf("posting PR comment: %w", err)
	}

	slog.Info("PR comment posted", "pr", prNumber)
	return nil
}
