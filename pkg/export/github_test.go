package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGHUsername(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "standard output",
			output:   "  Logged in to github.com account testuser (oauth_token)",
			expected: "testuser",
		},
		{
			name:     "with parenthetical",
			output:   "github.com\n  Logged in to github.com account myuser (keyring)",
			expected: "myuser",
		},
		{
			name:     "no account info",
			output:   "Not logged in to github.com",
			expected: "",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseGHUsername(tc.output)
			if result != tc.expected {
				t.Errorf("parseGHUsername(%q) = %q, want %q", tc.output, result, tc.expected)
			}
		})
	}
}

func TestSuggestRepoName(t *testing.T) {
	tests := []struct {
		bundlePath string
		expected   string
	}{
		{"/home/user/myproject/bv-pages", "myproject-pages"},
		{"/home/user/awesome-app/docs", "awesome-app-pages"},
		{"/home/user/My Project", "my-project"},
		{"./output", "output"},
	}

	for _, tc := range tests {
		t.Run(tc.bundlePath, func(t *testing.T) {
			result := SuggestRepoName(tc.bundlePath)
			if result != tc.expected {
				t.Errorf("SuggestRepoName(%q) = %q, want %q", tc.bundlePath, result, tc.expected)
			}
		})
	}
}

func TestGitHubDeployConfig(t *testing.T) {
	config := GitHubDeployConfig{
		RepoName:         "test-repo",
		Private:          true,
		Description:      "Test description",
		BundlePath:       "/tmp/test",
		SkipConfirmation: true,
		ForceOverwrite:   false,
	}

	if config.RepoName != "test-repo" {
		t.Errorf("Expected RepoName 'test-repo', got %s", config.RepoName)
	}

	if !config.Private {
		t.Error("Expected Private to be true")
	}

	if config.ForceOverwrite {
		t.Error("Expected ForceOverwrite to be false")
	}
}

func TestGitHubDeployResult(t *testing.T) {
	result := GitHubDeployResult{
		RepoFullName: "user/repo",
		PagesURL:     "https://user.github.io/repo/",
		GitRemote:    "https://github.com/user/repo.git",
	}

	if result.RepoFullName != "user/repo" {
		t.Errorf("Expected RepoFullName 'user/repo', got %s", result.RepoFullName)
	}

	if result.PagesURL != "https://user.github.io/repo/" {
		t.Errorf("Expected PagesURL 'https://user.github.io/repo/', got %s", result.PagesURL)
	}
}

func TestGitHubStatus(t *testing.T) {
	status := GitHubStatus{
		Installed:     true,
		Authenticated: true,
		Username:      "testuser",
		GitConfigured: true,
		GitName:       "Test User",
		GitEmail:      "test@example.com",
	}

	if !status.Installed {
		t.Error("Expected Installed to be true")
	}

	if !status.Authenticated {
		t.Error("Expected Authenticated to be true")
	}

	if status.Username != "testuser" {
		t.Errorf("Expected Username 'testuser', got %s", status.Username)
	}

	if !status.GitConfigured {
		t.Error("Expected GitConfigured to be true")
	}
}

func TestGitHubPagesStatus(t *testing.T) {
	status := GitHubPagesStatus{
		Enabled:   true,
		URL:       "https://user.github.io/repo/",
		Branch:    "main",
		Path:      "/",
		BuildType: "legacy",
	}

	if !status.Enabled {
		t.Error("Expected Enabled to be true")
	}

	if status.URL != "https://user.github.io/repo/" {
		t.Errorf("Expected URL 'https://user.github.io/repo/', got %s", status.URL)
	}

	if status.Branch != "main" {
		t.Errorf("Expected Branch 'main', got %s", status.Branch)
	}
}

func TestCheckGHStatus_Integration(t *testing.T) {
	// This test checks if the function runs without panic
	// Actual values depend on the environment
	status, err := CheckGHStatus()
	if err != nil {
		t.Errorf("CheckGHStatus() returned error: %v", err)
	}

	if status == nil {
		t.Fatal("CheckGHStatus() returned nil status")
	}

	// Log the status for debugging (won't fail test)
	t.Logf("gh CLI installed: %v", status.Installed)
	t.Logf("gh authenticated: %v", status.Authenticated)
	t.Logf("git configured: %v", status.GitConfigured)
}

func TestInitAndPush_MissingBundlePath(t *testing.T) {
	// Test with non-existent path should fail gracefully
	err := InitAndPush("/nonexistent/path/12345", "user/repo", false)
	if err == nil {
		t.Error("Expected error for non-existent bundle path")
	}
}

func TestDeployToGitHubPages_MissingBundlePath(t *testing.T) {
	config := GitHubDeployConfig{
		RepoName:         "test-repo",
		BundlePath:       "/nonexistent/path/12345",
		SkipConfirmation: true,
	}

	_, err := DeployToGitHubPages(config)
	if err == nil {
		t.Error("Expected error for non-existent bundle path")
	}
}

func TestRepoExists_InvalidRepo(t *testing.T) {
	// This will return false for a non-existent repo
	exists := RepoExists("definitely-not-a-real-repo-12345-xyzzy")
	if exists {
		t.Error("Expected RepoExists to return false for non-existent repo")
	}
}

func TestGetRepoFullName_WithOwner(t *testing.T) {
	// If name already contains owner, should return as-is
	name, err := getRepoFullName("owner/repo")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if name != "owner/repo" {
		t.Errorf("Expected 'owner/repo', got %s", name)
	}
}

func TestShowInstallInstructions(t *testing.T) {
	// Just verify it doesn't panic
	// Capture stdout is complex, so we just ensure no crash
	ShowInstallInstructions()
}

func TestSuggestRepoName_CurrentDir(t *testing.T) {
	// Create a temp directory to test with
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "test-project", "bv-pages")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	result := SuggestRepoName(subDir)
	// Should use parent project name + -pages
	if result != "test-project-pages" {
		t.Errorf("Expected 'test-project-pages', got %s", result)
	}
}

func TestSuggestRepoName_RegularDir(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "my-static-site")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	result := SuggestRepoName(subDir)
	if result != "my-static-site" {
		t.Errorf("Expected 'my-static-site', got %s", result)
	}
}

func TestWriteGitHubActionsWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	// Write the workflow
	if err := WriteGitHubActionsWorkflow(tmpDir); err != nil {
		t.Fatalf("WriteGitHubActionsWorkflow failed: %v", err)
	}

	// Check that the workflow file exists
	workflowPath := filepath.Join(tmpDir, ".github", "workflows", "static.yml")
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		t.Errorf("Workflow file was not created at %s", workflowPath)
	}

	// Read and verify content
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("Failed to read workflow file: %v", err)
	}

	// Verify key workflow elements
	contentStr := string(content)
	checks := []string{
		"name: Deploy static content to Pages",
		"push:",
		"branches: [\"main\"]",
		"workflow_dispatch:",
		"actions/checkout@v4",
		"actions/configure-pages@v5",
		"actions/upload-pages-artifact@v3",
		"actions/deploy-pages@v4",
	}

	for _, check := range checks {
		if !strings.Contains(contentStr, check) {
			t.Errorf("Workflow missing expected content: %s", check)
		}
	}
}

func TestGitHubActionsWorkflowContent(t *testing.T) {
	// Verify the workflow content is valid YAML-like structure
	content := GitHubActionsWorkflowContent

	// Check required fields
	requiredFields := []string{
		"name:",
		"on:",
		"permissions:",
		"jobs:",
		"deploy:",
		"steps:",
	}

	for _, field := range requiredFields {
		if !strings.Contains(content, field) {
			t.Errorf("GitHubActionsWorkflowContent missing required field: %s", field)
		}
	}
}
