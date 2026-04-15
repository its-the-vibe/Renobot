package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_RequiredFields(t *testing.T) {
	t.Run("missing org returns error", func(t *testing.T) {
		path := writeConfig(t, "channel: \"#renovate\"\n")
		_, err := loadConfig(path)
		if err == nil {
			t.Fatal("expected error for missing org, got nil")
		}
	})

	t.Run("missing channel returns error", func(t *testing.T) {
		path := writeConfig(t, "org: myorg\n")
		_, err := loadConfig(path)
		if err == nil {
			t.Fatal("expected error for missing channel, got nil")
		}
	})
}

func TestLoadConfig_Defaults(t *testing.T) {
	path := writeConfig(t, "org: myorg\nchannel: \"#renovate\"\n")
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Cron != "0 9 * * 1-5" {
		t.Errorf("default cron = %q, want %q", cfg.Cron, "0 9 * * 1-5")
	}
	if cfg.RevampPath != "revamp" {
		t.Errorf("default revamp_path = %q, want %q", cfg.RevampPath, "revamp")
	}
	if cfg.Redis.Addr != "localhost:6379" {
		t.Errorf("default redis.addr = %q, want %q", cfg.Redis.Addr, "localhost:6379")
	}
	if cfg.Redis.ListKey != "slack_messages" {
		t.Errorf("default redis.list_key = %q, want %q", cfg.Redis.ListKey, "slack_messages")
	}
}

func TestLoadConfig_EnvExpansion(t *testing.T) {
	t.Setenv("TEST_ORG", "expanded-org")
	path := writeConfig(t, "org: ${TEST_ORG}\nchannel: \"#ch\"\n")
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Org != "expanded-org" {
		t.Errorf("org = %q, want expanded-org", cfg.Org)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := loadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadConfig_ExplicitValues(t *testing.T) {
	content := `
org: myorg
channel: "#renovate"
cron: "0 8 * * *"
revamp_path: /usr/local/bin/revamp
redis:
  addr: redis:6379
  db: 1
  list_key: my_messages
`
	path := writeConfig(t, content)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Org != "myorg" {
		t.Errorf("org = %q, want myorg", cfg.Org)
	}
	if cfg.Channel != "#renovate" {
		t.Errorf("channel = %q, want #renovate", cfg.Channel)
	}
	if cfg.Cron != "0 8 * * *" {
		t.Errorf("cron = %q, want 0 8 * * *", cfg.Cron)
	}
	if cfg.RevampPath != "/usr/local/bin/revamp" {
		t.Errorf("revamp_path = %q, want /usr/local/bin/revamp", cfg.RevampPath)
	}
	if cfg.Redis.Addr != "redis:6379" {
		t.Errorf("redis.addr = %q, want redis:6379", cfg.Redis.Addr)
	}
	if cfg.Redis.DB != 1 {
		t.Errorf("redis.db = %d, want 1", cfg.Redis.DB)
	}
	if cfg.Redis.ListKey != "my_messages" {
		t.Errorf("redis.list_key = %q, want my_messages", cfg.Redis.ListKey)
	}
}

// writeConfig writes content to a temp file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}
