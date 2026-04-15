package main

import (
	"testing"
)

func TestParseBranchOutput(t *testing.T) {
	input := `     6 renovate/golang-version-updates
     3 renovate/github.com-github-copilot-sdk-go-0.x
     2 renovate/cloud.google.com-go-firestore-1.x
     1 renovate/cloud.google.com-go-pubsub-v2-2.x
     1 renovate/google.golang.org-api-0.x
`
	got := parseBranchOutput(input)
	want := []BranchSummary{
		{6, "renovate/golang-version-updates"},
		{3, "renovate/github.com-github-copilot-sdk-go-0.x"},
		{2, "renovate/cloud.google.com-go-firestore-1.x"},
		{1, "renovate/cloud.google.com-go-pubsub-v2-2.x"},
		{1, "renovate/google.golang.org-api-0.x"},
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestParseBranchOutput_Empty(t *testing.T) {
	if got := parseBranchOutput(""); len(got) != 0 {
		t.Errorf("expected empty slice, got %+v", got)
	}
}

func TestParseBranchOutput_SkipsInvalidLines(t *testing.T) {
	input := "not-a-number branch\n5 valid/branch\n"
	got := parseBranchOutput(input)
	if len(got) != 1 || got[0].Branch != "valid/branch" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestParseRepoOutput(t *testing.T) {
	input := `its-the-vibe/Call2Action
its-the-vibe/ClassiFiler
its-the-vibe/EventHorizon
`
	got := parseRepoOutput(input)
	want := []string{"its-the-vibe/Call2Action", "its-the-vibe/ClassiFiler", "its-the-vibe/EventHorizon"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestParseRepoOutput_Empty(t *testing.T) {
	if got := parseRepoOutput(""); len(got) != 0 {
		t.Errorf("expected empty slice, got %+v", got)
	}
}

func TestParseRepoOutput_SkipsBlankLines(t *testing.T) {
	input := "\nrepo-a\n\nrepo-b\n\n"
	got := parseRepoOutput(input)
	if len(got) != 2 {
		t.Errorf("expected 2 repos, got %d: %+v", len(got), got)
	}
}
