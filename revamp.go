package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// BranchSummary represents a single line from `revamp summary --branch` output.
type BranchSummary struct {
	Count  int
	Branch string
}

// runRevampBranches executes `revamp summary --org <org> --branch` and returns
// the parsed list of branch summaries.
func runRevampBranches(revampPath, org string) ([]BranchSummary, error) {
	out, err := runCommand(revampPath, "summary", "--org", org, "--branch")
	if err != nil {
		return nil, fmt.Errorf("revamp summary --branch: %w", err)
	}
	return parseBranchOutput(out), nil
}

// runRevampRepos executes `revamp summary --org <org> --head <branch>` and
// returns the newline-separated list of repository names.
func runRevampRepos(revampPath, org, branch string) ([]string, error) {
	out, err := runCommand(revampPath, "summary", "--org", org, "--head", branch)
	if err != nil {
		return nil, fmt.Errorf("revamp summary --head %s: %w", branch, err)
	}
	return parseRepoOutput(out), nil
}

// runCommand runs an external command and returns its combined output.
func runCommand(name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w\nstderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// parseBranchOutput parses the output of `revamp summary --branch`.
// Each line has the format: "<count> <branch>".
func parseBranchOutput(output string) []BranchSummary {
	var results []BranchSummary
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		count, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		results = append(results, BranchSummary{
			Count:  count,
			Branch: fields[1],
		})
	}
	return results
}

// parseRepoOutput parses the output of `revamp summary --head <branch>`.
// Each non-empty line is a repository name.
func parseRepoOutput(output string) []string {
	var repos []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			repos = append(repos, line)
		}
	}
	return repos
}
