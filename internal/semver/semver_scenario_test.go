package semver

// Scenario tests exercise the full versioning lifecycle using real in-process
// git repositories (git.PlainInit in a temp dir). Each test is independent.
//
// Squash-merge simulation
// ───────────────────────
// go-git's Worktree.Commit does not perform a git merge. To simulate a squash
// merge we:
//   1. Check out the target branch (main).
//   2. Write the same file changes that the source branch introduced.
//   3. Commit with a CC-typed message (e.g. "feat: …" or "fix: …").
//   4. Set SOURCE_BRANCH env var so IsMainlineMerge detects the source.
//
// Regular-merge simulation (used for develop-sync checks)
// ────────────────────────────────────────────────────────
// We use CommitOptions.Parents to attach the source-branch tip as a second
// parent of the merge commit. This makes the source commit reachable from the
// target branch's history, which is what IsMainlineSynced checks.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func scenarioRepo(t *testing.T) (*git.Repository, string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	return repo, dir
}

func addCommit(t *testing.T, repo *git.Repository, dir, filename, message string) plumbing.Hash {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(message), 0644); err != nil {
		t.Fatalf("WriteFile %q: %v", filename, err)
	}
	wt, _ := repo.Worktree()
	if _, err := wt.Add(filename); err != nil {
		t.Fatalf("Add %q: %v", filename, err)
	}
	hash, err := wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{Name: "ci", Email: "ci@test", When: time.Now()},
	})
	if err != nil {
		t.Fatalf("Commit %q: %v", message, err)
	}
	return hash
}

// addMergeCommit creates a commit with two parents: the current HEAD plus
// secondParent. This is how we simulate a regular (non-squash) merge so that
// secondParent becomes reachable from the target branch's history.
func addMergeCommit(t *testing.T, repo *git.Repository, dir, filename, message string, secondParent plumbing.Hash) plumbing.Hash {
	t.Helper()
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(message), 0644); err != nil {
		t.Fatalf("WriteFile %q: %v", filename, err)
	}
	wt, _ := repo.Worktree()
	if _, err := wt.Add(filename); err != nil {
		t.Fatalf("Add %q: %v", filename, err)
	}
	hash, err := wt.Commit(message, &git.CommitOptions{
		Author:  &object.Signature{Name: "ci", Email: "ci@test", When: time.Now()},
		Parents: []plumbing.Hash{head.Hash(), secondParent},
	})
	if err != nil {
		t.Fatalf("MergeCommit %q: %v", message, err)
	}
	return hash
}

func createBranch(t *testing.T, repo *git.Repository, name string) {
	t.Helper()
	head, _ := repo.Head()
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(name), head.Hash())
	if err := repo.Storer.SetReference(ref); err != nil {
		t.Fatalf("create branch %q: %v", name, err)
	}
}

func checkoutBranch(t *testing.T, repo *git.Repository, name string) {
	t.Helper()
	wt, _ := repo.Worktree()
	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(name),
		Force:  true,
	}); err != nil {
		t.Fatalf("checkout %q: %v", name, err)
	}
}

// ── Scenario 1: Fresh repo — no tags ─────────────────────────────────────────

// TestScenario_FreshRepo verifies that a brand-new repo without any semver tag
// causes GetLatestSemverTag to return an error, which is the signal for the
// caller to fall back to GitVersion.yml.
func TestScenario_FreshRepo(t *testing.T) {
	// Arrange
	repo, _ := scenarioRepo(t)

	// Act
	_, _, err := GetLatestSemverTag(repo)

	// Assert
	if err == nil {
		t.Error("expected error for repo with no semver tag, got nil")
	}
}

// ── Scenario 2: Feature branches squash-merged into develop ──────────────────

// TestScenario_FeaturesToDevelop simulates the inner development loop:
//
//	feature/login  ─┐ squash merge
//	feature/dash   ─┴─→  develop  (two squashed commits)
//
// On develop there are no version tags; BumpByCommits gives the pre-release
// version bump that CI can use for build metadata.
func TestScenario_FeaturesToDevelop(t *testing.T) {
	// Arrange
	repo, dir := scenarioRepo(t)
	addCommit(t, repo, dir, "init.txt", "initial commit")
	base := SemVer{1, 0, 0}
	if err := CreateVersionTag(repo, base); err != nil {
		t.Fatalf("tag v1.0.0: %v", err)
	}
	createBranch(t, repo, "develop")
	checkoutBranch(t, repo, "develop")
	addCommit(t, repo, dir, "login.txt", "feat: add login page")
	addCommit(t, repo, dir, "dash.txt", "fix: fix dashboard crash")

	_, tagHash, err := GetLatestSemverTag(repo)
	if err != nil {
		t.Fatalf("GetLatestSemverTag: %v", err)
	}
	msgs, err := GetCommitMessagesSinceTag(repo, tagHash)
	if err != nil {
		t.Fatalf("GetCommitMessagesSinceTag: %v", err)
	}

	// Act
	v := base
	BumpByCommits(&v, msgs)

	// Assert: feat wins over fix → Minor bump; no auto-tag created on develop
	if v != (SemVer{1, 1, 0}) {
		t.Errorf("expected v1.1.0 on develop, got %+v", v)
	}
	tags, _ := repo.Tags()
	count := 0
	_ = tags.ForEach(func(_ *plumbing.Reference) error { count++; return nil })
	if count != 1 {
		t.Errorf("expected 1 tag on develop (no auto-tag), got %d", count)
	}
}

// ── Scenario 3: Squash merge develop → main  (commit-driven bump + tag) ─────

// TestScenario_SquashMergeDevelopToMain simulates the release cycle:
//
//	develop ──squash──→ main  →  commit-driven bump  →  tag v2.0.0
//
// SOURCE_BRANCH=develop is set by CI (squash merge has no "Merge branch" msg).
// The bump level comes from the CC type in the squash commit message.
func TestScenario_SquashMergeDevelopToMain(t *testing.T) {
	// Arrange
	repo, dir := scenarioRepo(t)
	mainBranch := "main"
	t.Setenv("MAINLINE_BRANCH", mainBranch)

	addCommit(t, repo, dir, "init.txt", "initial commit")
	createBranch(t, repo, mainBranch)
	checkoutBranch(t, repo, mainBranch)
	addCommit(t, repo, dir, "base.txt", "chore: base")
	v := SemVer{1, 0, 0}
	if err := CreateVersionTag(repo, v); err != nil {
		t.Fatalf("tag v1.0.0: %v", err)
	}

	createBranch(t, repo, "develop")
	checkoutBranch(t, repo, "develop")
	addCommit(t, repo, dir, "feat1.txt", "feat: user profile")
	addCommit(t, repo, dir, "feat2.txt", "feat: notifications")

	// Squash merge: one CC-typed commit on main; ! suffix → Major bump.
	checkoutBranch(t, repo, mainBranch)
	squashMsg := "feat!: complete platform authentication overhaul"
	addCommit(t, repo, dir, "squash.txt", squashMsg)
	t.Setenv("SOURCE_BRANCH", "develop")

	msgs := []string{squashMsg}

	// Act
	if !IsMainlineMerge(msgs) {
		t.Fatal("IsMainlineMerge should return true for SOURCE_BRANCH=develop")
	}
	BumpByCommits(&v, msgs)
	err := CreateVersionTag(repo, v)

	// Assert
	if v != (SemVer{2, 0, 0}) {
		t.Errorf("expected v2.0.0, got %+v", v)
	}
	if err != nil {
		t.Fatalf("CreateVersionTag v2.0.0: %v", err)
	}
	tagName, _, err := GetLatestSemverTag(repo)
	if err != nil || tagName != "v2.0.0" {
		t.Errorf("expected tag v2.0.0, got %q (err: %v)", tagName, err)
	}
}

// ── Scenario 4: Hotfix — merge to main (Patch) then sync back to develop ─────

// TestScenario_HotfixFlowAndSync simulates:
//
//	main ──branch──→ hotfix/critical ──squash──→ main  →  Patch bump  →  tag v1.0.1
//	                 hotfix/critical ──merge──→  develop               (sync)
//
// After the hotfix is tagged on main, IsMainlineSynced must return false until
// the hotfix branch is also merged (regular merge) into develop.
func TestScenario_HotfixFlowAndSync(t *testing.T) {
	// Arrange
	repo, dir := scenarioRepo(t)
	addCommit(t, repo, dir, "init.txt", "initial commit")
	v := SemVer{1, 0, 0}
	if err := CreateVersionTag(repo, v); err != nil {
		t.Fatalf("tag v1.0.0: %v", err)
	}

	createBranch(t, repo, "develop")
	checkoutBranch(t, repo, "develop")
	addCommit(t, repo, dir, "feat.txt", "feat: new dashboard")

	checkoutBranch(t, repo, "master") // go-git default branch name
	createBranch(t, repo, "hotfix/critical")
	checkoutBranch(t, repo, "hotfix/critical")
	hotfixTip := addCommit(t, repo, dir, "fix.txt", "fix: critical security patch")

	// Act: squash merge hotfix → main
	checkoutBranch(t, repo, "master")
	squashMsg := "fix: critical security patch"
	addCommit(t, repo, dir, "squash-hotfix.txt", squashMsg)
	t.Setenv("SOURCE_BRANCH", "hotfix/critical")

	msgs := []string{squashMsg}
	if !IsMainlineMerge(msgs) {
		t.Fatal("IsMainlineMerge should return true for SOURCE_BRANCH=hotfix/critical")
	}
	BumpByCommits(&v, msgs)
	if err := CreateVersionTag(repo, v); err != nil {
		t.Fatalf("tag v1.0.1: %v", err)
	}

	// Assert: Patch bump applied and tag created
	if v != (SemVer{1, 0, 1}) {
		t.Fatalf("expected Patch bump to v1.0.1, got %+v", v)
	}

	// Act: check sync before merging hotfix back to develop
	synced, err := IsMainlineSynced(repo, "develop", hotfixTip)

	// Assert: develop is out of sync
	if err != nil {
		t.Fatalf("IsMainlineSynced (before): %v", err)
	}
	if synced {
		t.Error("develop should NOT be synced before merging hotfix back")
	}

	// Act: regular merge hotfix → develop (preserves hotfixTip as second parent)
	checkoutBranch(t, repo, "develop")
	addMergeCommit(t, repo, dir, "merge-hotfix.txt", "Merge branch 'hotfix/critical' into develop", hotfixTip)

	// Assert: develop is now in sync
	synced, err = IsMainlineSynced(repo, "develop", hotfixTip)
	if err != nil {
		t.Fatalf("IsMainlineSynced (after): %v", err)
	}
	if !synced {
		t.Error("develop SHOULD be synced after merging hotfix back")
	}
}

// ── Scenario 5: Release branch — Minor bump + sync back to develop ────────────

// TestScenario_ReleaseFlowAndSync simulates:
//
//	develop ──branch──→ release/2.1 ──squash──→ main  →  Minor bump  →  tag v1.1.0
//	                    release/2.1 ──merge──→  develop               (sync)
//
// Any stabilisation commits on release/* must also land in develop.
func TestScenario_ReleaseFlowAndSync(t *testing.T) {
	// Arrange
	repo, dir := scenarioRepo(t)
	addCommit(t, repo, dir, "init.txt", "initial commit")
	v := SemVer{1, 0, 0}
	if err := CreateVersionTag(repo, v); err != nil {
		t.Fatalf("tag v1.0.0: %v", err)
	}

	createBranch(t, repo, "develop")
	checkoutBranch(t, repo, "develop")
	addCommit(t, repo, dir, "feat1.txt", "feat: payment flow")

	createBranch(t, repo, "release/2.1")
	checkoutBranch(t, repo, "release/2.1")
	releaseTip := addCommit(t, repo, dir, "fix-release.txt", "fix: edge case in payment")

	// Act: squash merge release/2.1 → main
	checkoutBranch(t, repo, "master")
	squashMsg := "feat: payment flow"
	addCommit(t, repo, dir, "squash-release.txt", squashMsg)
	t.Setenv("SOURCE_BRANCH", "release/2.1")

	msgs := []string{squashMsg}
	if !IsMainlineMerge(msgs) {
		t.Fatal("IsMainlineMerge should return true for SOURCE_BRANCH=release/2.1")
	}
	BumpByCommits(&v, msgs)
	if err := CreateVersionTag(repo, v); err != nil {
		t.Fatalf("tag v1.1.0: %v", err)
	}

	// Assert: Minor bump applied and tag created
	if v != (SemVer{1, 1, 0}) {
		t.Fatalf("expected Minor bump to v1.1.0, got %+v", v)
	}

	// Act: check sync before merging release branch back to develop
	synced, err := IsMainlineSynced(repo, "develop", releaseTip)

	// Assert: develop is out of sync
	if err != nil {
		t.Fatalf("IsMainlineSynced (before): %v", err)
	}
	if synced {
		t.Error("develop should NOT be synced before merging release branch back")
	}

	// Act: regular merge release/2.1 → develop
	checkoutBranch(t, repo, "develop")
	addMergeCommit(t, repo, dir, "merge-release.txt", "Merge branch 'release/2.1' into develop", releaseTip)

	// Assert: develop is now in sync
	synced, err = IsMainlineSynced(repo, "develop", releaseTip)
	if err != nil {
		t.Fatalf("IsMainlineSynced (after): %v", err)
	}
	if !synced {
		t.Error("develop SHOULD be synced after merging release branch back")
	}
}

// ── Scenario 6: MAINLINE_BRANCH env var changes mainline branch name ─────────

// TestScenario_CustomMainlineBranch verifies that the tool respects a custom
// MAINLINE_BRANCH value. If the CI uses "master" instead of "main", the same
// flow should work unchanged.
func TestScenario_CustomMainlineBranch(t *testing.T) {
	// Arrange
	t.Setenv("MAINLINE_BRANCH", "master")
	t.Setenv("SOURCE_BRANCH", "develop")
	v := SemVer{3, 1, 0}
	msgs := []string{"feat!: overhaul public API"}

	// Act
	mainline := GetMainlineBranch()
	isMerge := IsMainlineMerge(msgs)
	BumpByCommits(&v, msgs)

	// Assert
	if mainline != "master" {
		t.Errorf("expected mainline branch 'master', got %q", mainline)
	}
	if !isMerge {
		t.Fatal("IsMainlineMerge should return true for SOURCE_BRANCH=develop")
	}
	if v != (SemVer{4, 0, 0}) {
		t.Errorf("expected v4.0.0 with custom mainline branch, got %+v", v)
	}
}

// ── Scenario 7: DEVELOP_BRANCH env var changes integration branch name ────────

// TestScenario_CustomDevelopBranch verifies that when teams name their
// integration branch "test" (or anything else), DEVELOP_BRANCH is respected by
// both IsMainlineMerge (SOURCE_BRANCH path) and IsMainlineSynced.
func TestScenario_CustomDevelopBranch(t *testing.T) {
	// Arrange
	t.Setenv("DEVELOP_BRANCH", "test")

	// Assert: env var is read correctly
	if GetDevelopBranch() != "test" {
		t.Errorf("expected develop branch 'test'")
	}

	// Act: SOURCE_BRANCH path — "test" must be recognised as integration-branch merge
	t.Setenv("SOURCE_BRANCH", "test")
	msgs := []string{"feat: new analytics dashboard"}

	// Assert
	if !IsMainlineMerge(msgs) {
		t.Fatal("IsMainlineMerge should return true when SOURCE_BRANCH equals DEVELOP_BRANCH")
	}

	// Act: commit-message path — "Merge branch 'test'" must also be recognised
	t.Setenv("SOURCE_BRANCH", "")
	mergeMsg := []string{"Merge branch 'test' into 'main'"}

	// Assert
	if !IsMainlineMerge(mergeMsg) {
		t.Fatal("IsMainlineMerge should return true for merge message containing DEVELOP_BRANCH value")
	}
	if IsMainlineMerge([]string{"Merge branch 'develop' into 'main'"}) {
		t.Error("IsMainlineMerge should return false for 'develop' when DEVELOP_BRANCH=test")
	}

	// Arrange: real repo to verify IsMainlineSynced uses the custom branch name
	repo, dir := scenarioRepo(t)
	addCommit(t, repo, dir, "init.txt", "initial commit")
	createBranch(t, repo, "test")
	checkoutBranch(t, repo, "test")
	tipHash := addCommit(t, repo, dir, "feat.txt", "feat: something")

	// Act
	synced, err := IsMainlineSynced(repo, GetDevelopBranch(), tipHash)

	// Assert: tip is HEAD of "test" branch → must be in sync
	if err != nil {
		t.Fatalf("IsMainlineSynced: %v", err)
	}
	if !synced {
		t.Error("expected synced=true when tipHash is the HEAD of DEVELOP_BRANCH")
	}
}
