package main

import (
	"fmt"
	"os"

	"semver/internal/semver"
)

func main() {
	repo, err := semver.OpenRepo()
	if err != nil {
		fmt.Fprintln(os.Stderr, "not a git repository — run from a directory that contains .git")
		os.Exit(1)
	}

	tag, tagHash, tagErr := semver.GetLatestSemverTag(repo)

	// ── No tag found ──────────────────────────────────────────────────────────
	// Before the first release. Read next-version from GitVersion.yml.
	// On mainline with a recognised merge, create the initial tag so subsequent
	// runs have a baseline to diff against. IsMainlineMerge checks SOURCE_BRANCH
	// first — no commit messages are needed for the squash-merge CI path.
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
		if semver.GetCurrentBranch(repo) == semver.GetMainlineBranch() && semver.IsMainlineMerge([]string{}) {
			if err := semver.CreateVersionTag(repo, v); err != nil {
				fmt.Fprintln(os.Stderr, "warning: could not create initial tag:", err)
			}
		}
		semver.PrintJSON(semver.BuildVersionInfo(repo, v))
		return
	}

	// ── Tag found — parse it ──────────────────────────────────────────────────
	v, err := semver.ParseTagVersion(tag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not parse tag as semver:", tag, err)
		os.Exit(1)
	}

	// ── Collect commits since the tag ─────────────────────────────────────────
	messages, err := semver.GetCommitMessagesSinceTag(repo, tagHash)
	if err != nil || len(messages) == 0 {
		// No new commits — current tag is still the correct version.
		semver.PrintJSON(semver.BuildVersionInfo(repo, v))
		return
	}

	// ── mainline merge: detect merge, bump by commits, auto-tag ─────────────
	//   Bump level is always driven by Conventional Commits in the messages.
	//   The source branch (develop, release/*, hotfix/*) only controls whether
	//   a tag is created — not the magnitude of the bump.
	if semver.GetCurrentBranch(repo) == semver.GetMainlineBranch() && semver.IsMainlineMerge(messages) {
		semver.BumpByCommits(&v, messages)
		if err := semver.CreateVersionTag(repo, v); err != nil {
			fmt.Fprintln(os.Stderr, "warning: could not create tag:", err)
		}
		semver.PrintJSON(semver.BuildVersionInfo(repo, v))
		return
	}

	// ── Bump based on all commits since the tag ───────────────────────────────
	semver.BumpByCommits(&v, messages)
	semver.PrintJSON(semver.BuildVersionInfo(repo, v))
}
