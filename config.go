package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for Renobot.
type Config struct {
	// Org is the GitHub organisation to pass to revamp.
	Org string `yaml:"org"`

	// Channel is the Slack channel to post summaries to.
	Channel string `yaml:"channel"`

	// Cron is the cron expression that controls the run schedule.
	Cron string `yaml:"cron"`

	// RevampPath is the path (or name) of the revamp binary.
	RevampPath string `yaml:"revamp_path"`

	// Redis holds connection settings for the SlackLiner Redis queue.
	Redis struct {
		Addr    string `yaml:"addr"`
		DB      int    `yaml:"db"`
		ListKey string `yaml:"list_key"`
	} `yaml:"redis"`

	// Poppit holds settings for dispatching commands via Poppit.
	Poppit struct {
		// InputList is the Redis list Poppit reads command payloads from.
		InputList string `yaml:"input_list"`
		// OutputChannel is the Redis pub/sub channel Poppit publishes command output to.
		OutputChannel string `yaml:"output_channel"`
		// Repo is the GitHub repository identifier included in Poppit payloads.
		Repo string `yaml:"repo"`
		// Branch is the Git branch reference included in Poppit payloads.
		Branch string `yaml:"branch"`
		// BaseDir is the working directory Poppit uses when executing commands.
		BaseDir string `yaml:"base_dir"`
	} `yaml:"poppit"`
}

// loadConfig reads and parses the YAML config file at path.
// Environment variable references in the file (${VAR}) are expanded
// before parsing.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if cfg.Org == "" {
		return nil, fmt.Errorf("config: org is required")
	}
	if cfg.Channel == "" {
		return nil, fmt.Errorf("config: channel is required")
	}
	if cfg.Cron == "" {
		cfg.Cron = "0 9 * * 1-5"
	}
	if cfg.RevampPath == "" {
		cfg.RevampPath = "revamp"
	}
	if cfg.Redis.Addr == "" {
		cfg.Redis.Addr = "localhost:6379"
	}
	if cfg.Redis.ListKey == "" {
		cfg.Redis.ListKey = "slack_messages"
	}
	if cfg.Poppit.InputList == "" {
		cfg.Poppit.InputList = "poppit:notifications"
	}
	if cfg.Poppit.OutputChannel == "" {
		cfg.Poppit.OutputChannel = "poppit:command-output"
	}
	if cfg.Poppit.Repo == "" {
		cfg.Poppit.Repo = "its-the-vibe/Renobot"
	}
	if cfg.Poppit.Branch == "" {
		cfg.Poppit.Branch = "refs/heads/main"
	}
	if cfg.Poppit.BaseDir == "" {
		cfg.Poppit.BaseDir = "."
	}

	return &cfg, nil
}
