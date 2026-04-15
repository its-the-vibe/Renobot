package main

import (
	"strings"
	"testing"
)

func TestFormatMessage(t *testing.T) {
	summary := BranchSummary{Count: 3, Branch: "renovate/golang-version-updates"}
	repos := []string{"org/repo-a", "org/repo-b", "org/repo-c"}

	got := formatMessage(summary, repos)

	// First line must contain branch name and count
	if want := "*renovate/golang-version-updates* (3 open PRs)"; !strings.Contains(got, want) {
		t.Errorf("message missing header %q\nGot:\n%s", want, got)
	}
	for _, repo := range repos {
		if !strings.Contains(got, "• "+repo) {
			t.Errorf("message missing repo %q\nGot:\n%s", repo, got)
		}
	}
}

func TestFormatMessage_SinglePR(t *testing.T) {
	summary := BranchSummary{Count: 1, Branch: "renovate/branch"}
	repos := []string{"org/repo"}

	got := formatMessage(summary, repos)

	if want := "1 open PR)"; !strings.Contains(got, want) {
		t.Errorf("expected singular PR in %q, got:\n%s", want, got)
	}
}

func TestPluralS(t *testing.T) {
	if pluralS(1) != "" {
		t.Error("expected empty string for 1")
	}
	if pluralS(0) != "s" {
		t.Error("expected 's' for 0")
	}
	if pluralS(2) != "s" {
		t.Error("expected 's' for 2")
	}
}

