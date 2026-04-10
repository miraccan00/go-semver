package main

import (
	"fmt"
	"os"

	"new-semver/internal/semver"
)

func main() {
	if !semver.CheckGitRepoExists() {
		fmt.Fprintln(os.Stderr, "not a git repository — run from a directory that contains .git")
		os.Exit(1)
	}

	tag, tagErr := semver.GetLatestSemverTag()

	// ── No tag found ──────────────────────────────────────────────────────────
	// Before the first release. Output next-version from GitVersion.yml as-is;
	// the CI pipeline is expected to create the first tag (e.g. v1.0.0).
	if tagErr != nil {
		nextVer, err := semver.ReadNextVersionFromYML()
		if err != nil {
			fmt.Fprintln(os.Stderr, "no git tags found and GitVersion.yml is missing or unreadable:", err)
			os.Exit(1)
		}
		v, err := semver.ParseVersion(nextVer)
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid next-version in GitVersion.yml:", err)
			os.Exit(1)
		}
		semver.PrintJSON(semver.BuildVersionInfo(v))
		return
	}

	// ── Tag found — parse it ──────────────────────────────────────────────────
	v, err := semver.ParseTagVersion(tag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not parse tag as semver:", tag, err)
		os.Exit(1)
	}

	// ── Collect commits since the tag ─────────────────────────────────────────
	messages, err := semver.GetCommitMessagesSinceTag(tag)
	if err != nil || len(messages) == 0 {
		// No new commits — current tag is still the correct version.
		semver.PrintJSON(semver.BuildVersionInfo(v))
		return
	}

	// ── Bump based on all commits since the tag ───────────────────────────────
	semver.BumpByCommits(&v, messages)
	semver.PrintJSON(semver.BuildVersionInfo(v))
}
