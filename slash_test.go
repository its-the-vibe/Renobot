package main

import (
	"context"
	"encoding/json"
	"testing"
)

// TestHandleSlashCommand_UnknownCommand verifies that non-/renobot commands
// are silently ignored without panicking.
func TestHandleSlashCommand_UnknownCommand(t *testing.T) {
	cfg := &Config{
		Org:        "myorg",
		Channel:    "#renovate",
		RevampPath: "revamp",
	}
	cfg.Poppit.InputList = "poppit:notifications"

	cmd := SlackCommand{
		Command:   "/other-command",
		Text:      "",
		ChannelID: "C12345",
		UserID:    "U99999",
	}
	raw, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	// nil rdb is safe here: the handler returns before reaching Redis for
	// non-/renobot commands.
	handleSlashCommand(context.Background(), cfg, nil, string(raw))
}

// TestHandleSlashCommand_InvalidJSON verifies that malformed payloads are
// handled gracefully without panicking.
func TestHandleSlashCommand_InvalidJSON(t *testing.T) {
	cfg := &Config{Org: "myorg", RevampPath: "revamp"}
	cfg.Poppit.InputList = "poppit:notifications"

	// nil rdb is safe: the handler returns on parse failure before using Redis.
	handleSlashCommand(context.Background(), cfg, nil, `{invalid json}`)
}

// TestSlackCommand_Parsing verifies that the SlackCommand struct can be
// correctly deserialised from a typical JSON payload.
func TestSlackCommand_Parsing(t *testing.T) {
	raw := `{
		"command":    "/renobot",
		"text":       "",
		"channel_id": "C99999",
		"user_id":    "U12345"
	}`

	var cmd SlackCommand
	if err := json.Unmarshal([]byte(raw), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cmd.Command != "/renobot" {
		t.Errorf("Command = %q, want /renobot", cmd.Command)
	}
	if cmd.ChannelID != "C99999" {
		t.Errorf("ChannelID = %q, want C99999", cmd.ChannelID)
	}
	if cmd.UserID != "U12345" {
		t.Errorf("UserID = %q, want U12345", cmd.UserID)
	}
}

// TestSlackCommand_Parsing_WithText verifies that the Text field is correctly
// deserialised when present.
func TestSlackCommand_Parsing_WithText(t *testing.T) {
	raw := `{"command":"/renobot","text":"some args","channel_id":"C1","user_id":"U1"}`

	var cmd SlackCommand
	if err := json.Unmarshal([]byte(raw), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cmd.Text != "some args" {
		t.Errorf("Text = %q, want %q", cmd.Text, "some args")
	}
}
