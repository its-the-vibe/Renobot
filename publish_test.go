package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
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
		if !strings.Contains(got, "• `"+repo+"`") {
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

func TestSlackMessage_TTLFields(t *testing.T) {
	ttl := 12 * time.Hour
	summary := BranchSummary{Count: 1, Branch: "renovate/branch"}
	repos := []string{"org/repo"}

	text := formatMessage(summary, repos)
	ttlSeconds := int64(ttl.Seconds())

	msg := slackMessage{
		Channel: "#test",
		Text:    text,
		TTL:     ttlSeconds,
		Metadata: &messageMetadata{
			EventType: "renobot",
			EventPayload: map[string]interface{}{
				"type":   "renobot",
				"branch": summary.Branch,
			},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	// SlackLiner expects a top-level "ttl" field in seconds.
	if got, ok := decoded["ttl"].(float64); !ok || int64(got) != ttlSeconds {
		t.Errorf("ttl = %v, want %d", decoded["ttl"], ttlSeconds)
	}

	// Metadata event_payload must NOT contain a duplicate ttl_seconds key.
	meta, ok := decoded["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("missing metadata field")
	}
	payload, ok := meta["event_payload"].(map[string]interface{})
	if !ok {
		t.Fatal("missing event_payload field")
	}
	if _, present := payload["ttl_seconds"]; present {
		t.Error("event_payload should not contain ttl_seconds; TTL belongs at top level")
	}
}
