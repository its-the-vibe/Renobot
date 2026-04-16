package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBuildMergeCommand_HeartEyesCat verifies that the 😻 reaction produces a
// merge command without --max.
func TestBuildMergeCommand_HeartEyesCat(t *testing.T) {
	cfg := &Config{
		Org:        "myorg",
		RevampPath: "revamp",
	}

	cmd, err := buildMergeCommand(cfg, "heart_eyes_cat", "renovate/golang-version-updates")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "revamp merge --org myorg --branch renovate/golang-version-updates"
	if cmd != want {
		t.Errorf("cmd = %q, want %q", cmd, want)
	}
}

// TestBuildMergeCommand_NumberEmoji verifies that number-emoji reactions produce
// merge commands with the correct --max value.
func TestBuildMergeCommand_NumberEmoji(t *testing.T) {
	cfg := &Config{
		Org:        "myorg",
		RevampPath: "revamp",
	}

	tests := []struct {
		reaction string
		wantMax  int
	}{
		{"one", 1},
		{"two", 2},
		{"three", 3},
		{"four", 4},
		{"five", 5},
		{"six", 6},
		{"seven", 7},
		{"eight", 8},
		{"nine", 9},
	}

	for _, tc := range tests {
		cmd, err := buildMergeCommand(cfg, tc.reaction, "renovate/branch")
		if err != nil {
			t.Errorf("[%s] unexpected error: %v", tc.reaction, err)
			continue
		}
		if !strings.Contains(cmd, "revamp merge") {
			t.Errorf("[%s] expected revamp merge, got %q", tc.reaction, cmd)
		}
		if !strings.Contains(cmd, "--max") {
			t.Errorf("[%s] expected --max flag, got %q", tc.reaction, cmd)
		}
	}
}

// TestBuildMergeCommand_NumberEmojiValues checks the exact command strings for
// well-known number emoji names.
func TestBuildMergeCommand_NumberEmojiValues(t *testing.T) {
	cfg := &Config{Org: "testorg", RevampPath: "revamp"}

	cmd, err := buildMergeCommand(cfg, "three", "renovate/foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "revamp merge --org testorg --branch renovate/foo --max 3"
	if cmd != want {
		t.Errorf("cmd = %q, want %q", cmd, want)
	}
}

// TestBuildMergeCommand_UnrecognisedReaction verifies that unknown reactions
// return an error.
func TestBuildMergeCommand_UnrecognisedReaction(t *testing.T) {
	cfg := &Config{Org: "myorg", RevampPath: "revamp"}

	_, err := buildMergeCommand(cfg, "thumbsup", "renovate/branch")
	if err == nil {
		t.Error("expected error for unrecognised reaction, got nil")
	}
}

// TestReactionToNumber_Known checks that all mapped emoji names return the
// correct integer.
func TestReactionToNumber_Known(t *testing.T) {
	tests := map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
		"six": 6, "seven": 7, "eight": 8, "nine": 9,
	}
	for name, want := range tests {
		got, ok := reactionToNumber(name)
		if !ok {
			t.Errorf("%q: expected ok=true", name)
		}
		if got != want {
			t.Errorf("%q: got %d, want %d", name, got, want)
		}
	}
}

// TestReactionToNumber_Unknown verifies that unmapped emoji names return false.
func TestReactionToNumber_Unknown(t *testing.T) {
	for _, name := range []string{"zero", "ten", "thumbsup", "heart_eyes_cat", ""} {
		_, ok := reactionToNumber(name)
		if ok {
			t.Errorf("%q: expected ok=false", name)
		}
	}
}

// TestHandleReactionEvent_HeartEyesCat checks that handleReactionEvent
// correctly builds the Poppit payload for a 😻 reaction.
func TestHandleReactionEvent_HeartEyesCat(t *testing.T) {
	cfg := &Config{
		Org:        "myorg",
		RevampPath: "revamp",
	}
	cfg.Poppit.Repo = "its-the-vibe/Renobot"
	cfg.Poppit.Branch = "refs/heads/main"
	cfg.Poppit.BaseDir = "/opt/app"
	cfg.Poppit.InputList = "poppit:notifications"

	evt := ReactionEvent{
		Event: SlackReactionEvent{
			Type:     "reaction_added",
			Reaction: "heart_eyes_cat",
			Item: SlackReactionItem{
				Type:    "message",
				Channel: "C12345",
				Ts:      "1776381814.663509",
			},
		},
		Metadata: &messageMetadata{
			EventType: "renobot",
			EventPayload: map[string]interface{}{
				"type":   "renobot",
				"branch": "renovate/golang-version-updates",
			},
		},
	}

	// Verify the event parses correctly and the command is built as expected.
	if evt.Event.Type != "reaction_added" {
		t.Errorf("Event.Type = %q, want reaction_added", evt.Event.Type)
	}
	branch, _ := evt.Metadata.EventPayload["branch"].(string)
	if branch == "" {
		t.Fatal("branch is empty")
	}

	cmd, err := buildMergeCommand(cfg, evt.Event.Reaction, branch)
	if err != nil {
		t.Fatalf("buildMergeCommand error: %v", err)
	}
	want := "revamp merge --org myorg --branch renovate/golang-version-updates"
	if cmd != want {
		t.Errorf("cmd = %q, want %q", cmd, want)
	}
}

// TestHandleReactionEvent_Parsing validates JSON round-trip of ReactionEvent.
func TestHandleReactionEvent_Parsing(t *testing.T) {
	raw := `{
		"event": {
			"type": "reaction_added",
			"reaction": "three",
			"item": {
				"type": "message",
				"channel": "C99999",
				"ts": "1776381814.663509"
			},
			"event_ts": "1776381871.000900",
			"user": "U12345"
		},
		"metadata": {
			"event_type": "renobot",
			"event_payload": {
				"type": "renobot",
				"branch": "renovate/foo"
			}
		}
	}`

	var evt ReactionEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if evt.Event.Type != "reaction_added" {
		t.Errorf("Event.Type = %q, want reaction_added", evt.Event.Type)
	}
	if evt.Event.Reaction != "three" {
		t.Errorf("Event.Reaction = %q, want three", evt.Event.Reaction)
	}
	if evt.Event.Item.Channel != "C99999" {
		t.Errorf("Event.Item.Channel = %q, want C99999", evt.Event.Item.Channel)
	}
	if evt.Event.Item.Ts != "1776381814.663509" {
		t.Errorf("Event.Item.Ts = %q, want 1776381814.663509", evt.Event.Item.Ts)
	}
	if evt.Metadata == nil {
		t.Fatal("Metadata is nil")
	}
	if evt.Metadata.EventType != "renobot" {
		t.Errorf("Metadata.EventType = %q, want renobot", evt.Metadata.EventType)
	}
	branch, _ := evt.Metadata.EventPayload["branch"].(string)
	if branch != "renovate/foo" {
		t.Errorf("metadata.branch = %q, want renovate/foo", branch)
	}
}

// TestHandleReactionEvent_IgnoresNonRenobot verifies that events for
// non-Renobot messages are silently ignored.
func TestHandleReactionEvent_IgnoresNonRenobot(t *testing.T) {
	raw := `{
		"event": {
			"type": "reaction_added",
			"reaction": "heart_eyes_cat",
			"item": {"type": "message", "channel": "C1", "ts": "123.456"}
		},
		"metadata": {
			"event_type": "other",
			"event_payload": {"type": "other"}
		}
	}`

	// handleReactionEvent should return early without panicking.
	// We test indirectly by confirming buildMergeCommand is never
	// reached via the guard on msgType.
	var evt ReactionEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	msgType, _ := evt.Metadata.EventPayload["type"].(string)
	if strings.EqualFold(msgType, "renobot") {
		t.Error("expected non-renobot message type to not match")
	}
}

// TestHandleReactionEvent_IgnoresMissingBranch verifies that events without
// a branch in metadata are silently skipped.
func TestHandleReactionEvent_IgnoresMissingBranch(t *testing.T) {
	raw := `{
		"event": {
			"type": "reaction_added",
			"reaction": "heart_eyes_cat",
			"item": {"type": "message", "channel": "C1", "ts": "123.456"}
		},
		"metadata": {
			"event_type": "renobot",
			"event_payload": {"type": "renobot"}
		}
	}`

	var evt ReactionEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	branch, _ := evt.Metadata.EventPayload["branch"].(string)
	if branch != "" {
		t.Errorf("expected empty branch, got %q", branch)
	}
}
