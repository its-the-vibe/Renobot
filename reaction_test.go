package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/slack-go/slack"
)

// mockSlackClient is a test double for SlackClient.
type mockSlackClient struct {
	messages []slack.Message
	err      error
}

func (m *mockSlackClient) GetConversationHistory(_ *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &slack.GetConversationHistoryResponse{Messages: m.messages}, nil
}

// TestBuildMergeCommand_HeartEyesCat verifies that the 😻 reaction produces a
// merge command without --max.
func TestBuildMergeCommand_HeartEyesCat(t *testing.T) {
	cfg := &Config{
		Org:        "myorg",
		RevampPath: "revamp",
	}

	cmd, err := buildRevampCommand(cfg, "heart_eyes_cat", "renovate/golang-version-updates")
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
		cmd, err := buildRevampCommand(cfg, tc.reaction, "renovate/branch")
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

	cmd, err := buildRevampCommand(cfg, "three", "renovate/foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "revamp merge --org testorg --branch renovate/foo --max 3"
	if cmd != want {
		t.Errorf("cmd = %q, want %q", cmd, want)
	}
}

// TestBuildMergeCommand_Hourglass verifies that the ⏳ reaction produces a
// list command with --head.
func TestBuildMergeCommand_Hourglass(t *testing.T) {
	cfg := &Config{
		Org:        "myorg",
		RevampPath: "revamp",
	}

	cmd, err := buildRevampCommand(cfg, "hourglass", "renovate/golang-version-updates")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "revamp list --org myorg --head renovate/golang-version-updates"
	if cmd != want {
		t.Errorf("cmd = %q, want %q", cmd, want)
	}
}

// TestBuildMergeCommand_UnrecognisedReaction verifies that unknown reactions
// return an error.
func TestBuildMergeCommand_UnrecognisedReaction(t *testing.T) {
	cfg := &Config{Org: "myorg", RevampPath: "revamp"}

	_, err := buildRevampCommand(cfg, "thumbsup", "renovate/branch")
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

// TestReactionEvent_Parsing validates JSON parsing of the raw Slack reaction event.
func TestReactionEvent_Parsing(t *testing.T) {
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
}

// TestFetchMessageMetadata_Renobot verifies that a message with renobot metadata
// is correctly returned by fetchMessageMetadata.
func TestFetchMessageMetadata_Renobot(t *testing.T) {
	msg := slack.Message{}
	msg.Metadata = slack.SlackMetadata{
		EventType: "renobot",
		EventPayload: map[string]interface{}{
			"type":   "renobot",
			"branch": "renovate/golang-version-updates",
		},
	}

	client := &mockSlackClient{messages: []slack.Message{msg}}
	meta, err := fetchMessageMetadata(client, "C12345", "1776381814.663509")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta == nil {
		t.Fatal("expected metadata, got nil")
	}
	if meta.EventType != "renobot" {
		t.Errorf("EventType = %q, want renobot", meta.EventType)
	}
	branch, _ := meta.EventPayload["branch"].(string)
	if branch != "renovate/golang-version-updates" {
		t.Errorf("branch = %q, want renovate/golang-version-updates", branch)
	}
}

// TestFetchMessageMetadata_NoMetadata verifies that a message without metadata
// returns nil without error.
func TestFetchMessageMetadata_NoMetadata(t *testing.T) {
	msg := slack.Message{} // no metadata set
	client := &mockSlackClient{messages: []slack.Message{msg}}
	meta, err := fetchMessageMetadata(client, "C12345", "1776381814.663509")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta != nil {
		t.Errorf("expected nil metadata, got %+v", meta)
	}
}

// TestFetchMessageMetadata_NotFound verifies that an empty message list
// returns nil without error.
func TestFetchMessageMetadata_NotFound(t *testing.T) {
	client := &mockSlackClient{messages: []slack.Message{}}
	meta, err := fetchMessageMetadata(client, "C12345", "9999999999.000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta != nil {
		t.Errorf("expected nil, got %+v", meta)
	}
}

// TestFetchMessageMetadata_APIError verifies that a Slack API error is propagated.
func TestFetchMessageMetadata_APIError(t *testing.T) {
	client := &mockSlackClient{err: errors.New("slack api error")}
	_, err := fetchMessageMetadata(client, "C12345", "1776381814.663509")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// TestFetchMessageMetadata_NonRenobot verifies that non-renobot metadata is
// returned as-is (filtering happens in handleReactionEvent, not here).
func TestFetchMessageMetadata_NonRenobot(t *testing.T) {
	msg := slack.Message{}
	msg.Metadata = slack.SlackMetadata{
		EventType: "other-service",
		EventPayload: map[string]interface{}{
			"type": "other",
		},
	}
	client := &mockSlackClient{messages: []slack.Message{msg}}
	meta, err := fetchMessageMetadata(client, "C12345", "1776381814.663509")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta == nil {
		t.Fatal("expected metadata, got nil")
	}
	if meta.EventType != "other-service" {
		t.Errorf("EventType = %q, want other-service", meta.EventType)
	}
}
