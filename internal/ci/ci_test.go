//nolint:testpackage // Test package uses internal package for access to private members
package ci_test

import (
	"context"
	"strings"
	"testing"

	"github.com/aurelbalteaux/infra-tools/internal/ci"
)

func TestBuildRunURL(t *testing.T) {
	tests := []struct {
		name      string
		serverURL string
		repo      string
		runID     string
		want      string
	}{
		{
			name:      "valid environment",
			serverURL: "https://github.com",
			repo:      "owner/repo",
			runID:     "12345",
			want:      "https://github.com/owner/repo/actions/runs/12345",
		},
		{
			name:      "missing server URL",
			serverURL: "",
			repo:      "owner/repo",
			runID:     "12345",
			want:      "",
		},
		{
			name:      "missing repo",
			serverURL: "https://github.com",
			repo:      "",
			runID:     "12345",
			want:      "",
		},
		{
			name:      "missing run ID",
			serverURL: "https://github.com",
			repo:      "owner/repo",
			runID:     "",
			want:      "",
		},
		{
			name:      "all missing",
			serverURL: "",
			repo:      "",
			runID:     "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set test env vars
			t.Setenv("GITHUB_SERVER_URL", tt.serverURL)
			t.Setenv("GITHUB_REPOSITORY", tt.repo)
			t.Setenv("GITHUB_RUN_ID", tt.runID)

			got := ci.BuildRunURL()
			if got != tt.want {
				t.Errorf("BuildRunURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPostPRComment_MissingEnvVars(t *testing.T) {
	// Clear env vars to trigger fallback
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_REPOSITORY", "")
	t.Setenv("PR_NUMBER", "")

	bodyBuilderCalled := false
	buildBody := func(runURL string) string {
		bodyBuilderCalled = true
		return "test comment body"
	}

	// This should not return an error, but fall back to printing
	err := ci.PostPRComment(context.Background(), "<!-- test-marker -->", buildBody)
	if err != nil {
		t.Errorf("PostPRComment() unexpected error: %v", err)
	}

	if !bodyBuilderCalled {
		t.Error("buildBody callback was not called")
	}
}

func TestPostPRComment_InvalidPRNumber(t *testing.T) {
	tests := []struct {
		name     string
		prNumber string
		wantErr  bool
	}{
		{
			name:     "non-numeric PR number",
			prNumber: "not-a-number",
			wantErr:  true,
		},
		{
			name:     "zero PR number",
			prNumber: "0",
			wantErr:  true,
		},
		{
			name:     "negative PR number",
			prNumber: "-1",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", "fake-token")
			t.Setenv("GITHUB_REPOSITORY", "owner/repo")
			t.Setenv("PR_NUMBER", tt.prNumber)

			buildBody := func(runURL string) string {
				return "test comment"
			}

			err := ci.PostPRComment(context.Background(), "<!-- test -->", buildBody)
			if (err != nil) != tt.wantErr {
				t.Errorf("PostPRComment() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), "invalid PR_NUMBER") {
					t.Errorf("expected error to contain 'invalid PR_NUMBER', got: %v", err)
				}
			}
		})
	}
}

func TestPostPRComment_BuildBodyCallback(t *testing.T) {
	// Clear env vars to trigger fallback (won't actually post)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_SERVER_URL", "https://github.com")
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	t.Setenv("GITHUB_RUN_ID", "12345")

	var receivedURL string
	buildBody := func(runURL string) string {
		receivedURL = runURL
		return "test body"
	}

	_ = ci.PostPRComment(context.Background(), "<!-- test -->", buildBody)

	expectedURL := "https://github.com/owner/repo/actions/runs/12345"
	if receivedURL != expectedURL {
		t.Errorf("buildBody received runURL = %q, want %q", receivedURL, expectedURL)
	}
}
