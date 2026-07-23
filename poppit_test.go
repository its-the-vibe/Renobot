package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildHeadPayloads(t *testing.T) {
	cfg := &Config{
		Org:        "myorg",
		RevampPath: "revamp",
	}
	cfg.Poppit.Repo = "my-org/my-repo"
	cfg.Poppit.Branch = "refs/heads/main"
	cfg.Poppit.BaseDir = "/opt/myapp"

	branchOutput := `     6 renovate/golang-version-updates
     3 renovate/github.com-github-copilot-sdk-go-0.x
`

	payloads := buildHeadPayloads(cfg, branchOutput)

	if len(payloads) != 2 {
		t.Fatalf("expected 2 payloads, got %d", len(payloads))
	}

	for i, tc := range []struct {
		branch string
		count  float64
		cmd    string
	}{
		{"renovate/golang-version-updates", 6, "revamp summary --org myorg --head renovate/golang-version-updates"},
		{"renovate/github.com-github-copilot-sdk-go-0.x", 3, "revamp summary --org myorg --head renovate/github.com-github-copilot-sdk-go-0.x"},
	} {
		p := payloads[i]
		if p.Type != "Renobot" {
			t.Errorf("[%d] Type = %q, want Renobot", i, p.Type)
		}
		if p.Repo != cfg.Poppit.Repo {
			t.Errorf("[%d] Repo = %q, want %q", i, p.Repo, cfg.Poppit.Repo)
		}
		if p.Branch != cfg.Poppit.Branch {
			t.Errorf("[%d] Branch = %q, want %q", i, p.Branch, cfg.Poppit.Branch)
		}
		if p.Dir != cfg.Poppit.BaseDir {
			t.Errorf("[%d] Dir = %q, want %q", i, p.Dir, cfg.Poppit.BaseDir)
		}
		if len(p.Commands) != 1 || p.Commands[0] != tc.cmd {
			t.Errorf("[%d] Commands = %v, want [%q]", i, p.Commands, tc.cmd)
		}
		branch, _ := p.Metadata["branch"].(string)
		if branch != tc.branch {
			t.Errorf("[%d] metadata.branch = %q, want %q", i, branch, tc.branch)
		}
		count, _ := p.Metadata["count"].(float64)
		if count != tc.count {
			t.Errorf("[%d] metadata.count = %v, want %v", i, count, tc.count)
		}
		metaType, _ := p.Metadata["type"].(string)
		if metaType != "Renobot" {
			t.Errorf("[%d] metadata.type = %q, want Renobot", i, metaType)
		}
	}
}

func TestBuildHeadPayloads_Empty(t *testing.T) {
	cfg := &Config{Org: "myorg", RevampPath: "revamp"}
	cfg.Poppit.Repo = "r"
	cfg.Poppit.Branch = "refs/heads/main"
	cfg.Poppit.BaseDir = "."

	payloads := buildHeadPayloads(cfg, "")
	if len(payloads) != 0 {
		t.Errorf("expected empty payloads, got %d", len(payloads))
	}
}

func TestPoppitPayload_JSONRoundTrip(t *testing.T) {
	p := PoppitPayload{
		Repo:     "its-the-vibe/Renobot",
		Branch:   "refs/heads/main",
		Type:     "Renobot",
		Dir:      "/opt/app",
		Commands: []string{"revamp summary --org myorg --branch"},
		Metadata: map[string]interface{}{"type": "Renobot"},
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var got PoppitPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if got.Repo != p.Repo {
		t.Errorf("Repo = %q, want %q", got.Repo, p.Repo)
	}
	if got.Type != p.Type {
		t.Errorf("Type = %q, want %q", got.Type, p.Type)
	}
	if len(got.Commands) != 1 || got.Commands[0] != p.Commands[0] {
		t.Errorf("Commands = %v, want %v", got.Commands, p.Commands)
	}
}

func TestFormatListReply_MultipleURLs(t *testing.T) {
	urls := []string{
		"https://github.com/org/repo-a/pull/1",
		"https://github.com/org/repo-b/pull/2",
		"https://github.com/org/repo-c/pull/3",
	}

	got := formatListReply("renovate/golang-version-updates", urls)

	if want := "Found 3 Renovate PRs queued for branch renovate/golang-version-updates"; !strings.Contains(got, want) {
		t.Errorf("expected header %q\nGot:\n%s", want, got)
	}
	for _, u := range urls {
		if !strings.Contains(got, "<"+u+">") {
			t.Errorf("expected URL %q in reply\nGot:\n%s", u, got)
		}
	}
}

func TestFormatListReply_SingleURL(t *testing.T) {
	urls := []string{"https://github.com/org/repo/pull/42"}

	got := formatListReply("renovate/branch", urls)

	if want := "Found 1 Renovate PR queued for branch renovate/branch"; !strings.Contains(got, want) {
		t.Errorf("expected singular form %q\nGot:\n%s", want, got)
	}
	if !strings.Contains(got, "<https://github.com/org/repo/pull/42>") {
		t.Errorf("expected URL in reply\nGot:\n%s", got)
	}
}

func TestFormatListReply_NoURLs(t *testing.T) {
	got := formatListReply("renovate/golang-version-updates", nil)

	want := "No Renovate PRs found for branch renovate/golang-version-updates"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatListReply_EmptyURLs(t *testing.T) {
	got := formatListReply("renovate/branch", []string{})

	want := "No Renovate PRs found for branch renovate/branch"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPoppitOutput_JSONUnmarshal(t *testing.T) {
	raw := `{
		"metadata": {"type": "Renobot", "branch": "renovate/foo", "count": 3},
		"type": "Renobot",
		"command": "revamp summary --org myorg --head renovate/foo",
		"output": "its-the-vibe/SomeRepo\n",
		"stderr": ""
	}`

	var out PoppitOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.Type != "Renobot" {
		t.Errorf("Type = %q, want Renobot", out.Type)
	}
	if !strings.Contains(out.Command, "--head") {
		t.Errorf("Command = %q, expected --head", out.Command)
	}
	branch, _ := out.Metadata["branch"].(string)
	if branch != "renovate/foo" {
		t.Errorf("metadata.branch = %q, want renovate/foo", branch)
	}
	count, _ := out.Metadata["count"].(float64)
	if count != 3 {
		t.Errorf("metadata.count = %v, want 3", count)
	}
}
