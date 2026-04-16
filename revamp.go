package main

import (
	"strconv"
	"strings"
)

// BranchSummary represents a single line from `revamp summary --branch` output.
type BranchSummary struct {
	Count  int
	Branch string
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
