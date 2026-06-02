package ci

import (
	"context"
	"os"
	"strings"
	"testing"
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
			// Save original env vars
			origServer := os.Getenv("GITHUB_SERVER_URL")
			origRepo := os.Getenv("GITHUB_REPOSITORY")
			origRun := os.Getenv("GITHUB_RUN_ID")
			defer func() {
				os.Setenv("GITHUB_SERVER_URL", origServer)
				os.Setenv("GITHUB_REPOSITORY", origRepo)
				os.Setenv("GITHUB_RUN_ID", origRun)
			}()

			// Set test env vars
			os.Setenv("GITHUB_SERVER_URL", tt.serverURL)
			os.Setenv("GITHUB_REPOSITORY", tt.repo)
			os.Setenv("GITHUB_RUN_ID", tt.runID)

			got := BuildRunURL()
			if got != tt.want {
				t.Errorf("BuildRunURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPostPRComment_MissingEnvVars(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("GITHUB_TOKEN")
	origRepo := os.Getenv("GITHUB_REPOSITORY")
	origPR := os.Getenv("PR_NUMBER")
	defer func() {
		os.Setenv("GITHUB_TOKEN", origToken)
		os.Setenv("GITHUB_REPOSITORY", origRepo)
		os.Setenv("PR_NUMBER", origPR)
	}()

	// Clear env vars to trigger fallback
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GITHUB_REPOSITORY")
	os.Unsetenv("PR_NUMBER")

	bodyBuilderCalled := false
	buildBody := func(runURL string) string {
		bodyBuilderCalled = true
		return "test comment body"
	}

	// This should not return an error, but fall back to printing
	err := PostPRComment(context.Background(), "<!-- test-marker -->", buildBody)
	if err != nil {
		t.Errorf("PostPRComment() unexpected error: %v", err)
	}

	if !bodyBuilderCalled {
		t.Error("buildBody callback was not called")
	}
}

func TestPostPRComment_InvalidPRNumber(t *testing.T) {
	// Save original env vars
	origToken := os.Getenv("GITHUB_TOKEN")
	origRepo := os.Getenv("GITHUB_REPOSITORY")
	origPR := os.Getenv("PR_NUMBER")
	defer func() {
		os.Setenv("GITHUB_TOKEN", origToken)
		os.Setenv("GITHUB_REPOSITORY", origRepo)
		os.Setenv("PR_NUMBER", origPR)
	}()

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
			os.Setenv("GITHUB_TOKEN", "fake-token")
			os.Setenv("GITHUB_REPOSITORY", "owner/repo")
			os.Setenv("PR_NUMBER", tt.prNumber)

			buildBody := func(runURL string) string {
				return "test comment"
			}

			err := PostPRComment(context.Background(), "<!-- test -->", buildBody)
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
	// Save original env vars
	origToken := os.Getenv("GITHUB_TOKEN")
	origRepo := os.Getenv("GITHUB_REPOSITORY")
	origPR := os.Getenv("PR_NUMBER")
	origServer := os.Getenv("GITHUB_SERVER_URL")
	origRunID := os.Getenv("GITHUB_RUN_ID")
	defer func() {
		os.Setenv("GITHUB_TOKEN", origToken)
		os.Setenv("GITHUB_REPOSITORY", origRepo)
		os.Setenv("PR_NUMBER", origPR)
		os.Setenv("GITHUB_SERVER_URL", origServer)
		os.Setenv("GITHUB_RUN_ID", origRunID)
	}()

	// Clear env vars to trigger fallback (won't actually post)
	os.Unsetenv("GITHUB_TOKEN")
	os.Setenv("GITHUB_SERVER_URL", "https://github.com")
	os.Setenv("GITHUB_REPOSITORY", "owner/repo")
	os.Setenv("GITHUB_RUN_ID", "12345")

	var receivedURL string
	buildBody := func(runURL string) string {
		receivedURL = runURL
		return "test body"
	}

	_ = PostPRComment(context.Background(), "<!-- test -->", buildBody)

	expectedURL := "https://github.com/owner/repo/actions/runs/12345"
	if receivedURL != expectedURL {
		t.Errorf("buildBody received runURL = %q, want %q", receivedURL, expectedURL)
	}
}
