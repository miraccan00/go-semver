package semver

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

var semverTagRE = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

// Conventional Commit regexes applied to the first line (subject) of a message.
// They support an optional scope: feat(scope): and an optional breaking ! marker.
var (
	reBreakingType = regexp.MustCompile(`(?i)^(feat|fix)(\([^)]*\))?!:`)
	reFeat         = regexp.MustCompile(`(?i)^feat(\([^)]*\))?:`)
	reFix          = regexp.MustCompile(`(?i)^fix(\([^)]*\))?:`)
)

// GetMainlineBranch returns the name of the production release branch.
// Reads the MAINLINE_BRANCH environment variable; defaults to "main".
func GetMainlineBranch() string {
	if b := os.Getenv("MAINLINE_BRANCH"); b != "" {
		return b
	}
	return "main"
}

// GetSourceBranch returns the name of the branch being merged into mainline,
// as declared by the CI system via the SOURCE_BRANCH environment variable.
// Returns an empty string when not set; the caller falls back to commit-message
// detection in that case.
//
// Set SOURCE_BRANCH in CI whenever a squash merge is used, because squash
// merges produce a plain commit with no "Merge branch …" message.
func GetSourceBranch() string {
	return os.Getenv("SOURCE_BRANCH")
}

// GetDevelopBranch returns the integration branch name.
// Reads the DEVELOP_BRANCH environment variable; defaults to "develop".
// Use this when the integration branch is named differently (e.g. "test").
func GetDevelopBranch() string {
	if b := os.Getenv("DEVELOP_BRANCH"); b != "" {
		return b
	}
	return "develop"
}

type SemVer struct {
	Major int
	Minor int
	Patch int
}

type VersionInfo struct {
	Major                     int    `json:"Major"`
	Minor                     int    `json:"Minor"`
	Patch                     int    `json:"Patch"`
	PreReleaseTag             string `json:"PreReleaseTag"`
	PreReleaseTagWithDash     string `json:"PreReleaseTagWithDash"`
	PreReleaseLabel           string `json:"PreReleaseLabel"`
	PreReleaseLabelWithDash   string `json:"PreReleaseLabelWithDash"`
	PreReleaseNumber          int    `json:"PreReleaseNumber"`
	WeightedPreReleaseNumber  int    `json:"WeightedPreReleaseNumber"`
	BuildMetaData             int    `json:"BuildMetaData"`
	FullBuildMetaData         string `json:"FullBuildMetaData"`
	MajorMinorPatch           string `json:"MajorMinorPatch"`
	SemVer                    string `json:"SemVer"`
	AssemblySemVer            string `json:"AssemblySemVer"`
	AssemblySemFileVer        string `json:"AssemblySemFileVer"`
	InformationalVersion      string `json:"InformationalVersion"`
	FullSemVer                string `json:"FullSemVer"`
	BranchName                string `json:"BranchName"`
	EscapedBranchName         string `json:"EscapedBranchName"`
	Sha                       string `json:"Sha"`
	ShortSha                  string `json:"ShortSha"`
	VersionSourceSha          string `json:"VersionSourceSha"`
	CommitsSinceVersionSource int    `json:"CommitsSinceVersionSource"`
	CommitDate                string `json:"CommitDate"`
	UncommittedChanges        int    `json:"UncommittedChanges"`
}

// OpenRepo opens the git repository rooted at the current working directory.
func OpenRepo() (*git.Repository, error) {
	return git.PlainOpen(".")
}

// CheckGitRepoExists returns true when the current directory is inside a git repo.
func CheckGitRepoExists() bool {
	_, err := git.PlainOpen(".")
	return err == nil
}

func ParseVersion(s string) (SemVer, error) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) != 3 {
		return SemVer{}, fmt.Errorf("invalid version format")
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])
	return SemVer{major, minor, patch}, nil
}

// ParseTagVersion strips an optional "v" prefix before parsing.
func ParseTagVersion(tag string) (SemVer, error) {
	return ParseVersion(strings.TrimPrefix(tag, "v"))
}

// GetLatestSemverTag walks the commit graph from HEAD and returns the nearest
// ancestor tag whose name matches a semver pattern (with or without "v" prefix).
// It handles both lightweight and annotated tags.
// Returns the tag name and the commit hash it resolves to.
func GetLatestSemverTag(repo *git.Repository) (string, plumbing.Hash, error) {
	// Build commit-hash → tag-name map for all semver tags.
	tagMap := make(map[plumbing.Hash]string)

	tagIter, err := repo.Tags()
	if err != nil {
		return "", plumbing.ZeroHash, fmt.Errorf("listing tags: %w", err)
	}
	err = tagIter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if !semverTagRE.MatchString(name) {
			return nil
		}
		// Annotated tag: ref points to a tag object, resolve to its commit.
		if tagObj, err := repo.TagObject(ref.Hash()); err == nil {
			commit, err := tagObj.Commit()
			if err != nil {
				return nil
			}
			tagMap[commit.Hash] = name
		} else {
			// Lightweight tag: ref points directly to a commit.
			tagMap[ref.Hash()] = name
		}
		return nil
	})
	if err != nil {
		return "", plumbing.ZeroHash, err
	}
	if len(tagMap) == 0 {
		return "", plumbing.ZeroHash, fmt.Errorf("no semver tag found")
	}

	// Walk from HEAD; the first tagged commit we encounter is the nearest ancestor.
	head, err := repo.Head()
	if err != nil {
		return "", plumbing.ZeroHash, fmt.Errorf("resolving HEAD: %w", err)
	}
	logIter, err := repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return "", plumbing.ZeroHash, err
	}

	var foundName string
	var foundHash plumbing.Hash
	_ = logIter.ForEach(func(c *object.Commit) error {
		if name, ok := tagMap[c.Hash]; ok {
			foundName = name
			foundHash = c.Hash
			return storer.ErrStop
		}
		return nil
	})

	if foundName == "" {
		return "", plumbing.ZeroHash, fmt.Errorf("no semver tag found in commit ancestry")
	}
	return foundName, foundHash, nil
}

// GetCommitMessagesSinceTag returns the full message of every commit reachable
// from HEAD up to (but not including) the commit at tagHash.
// Full messages are returned so that multi-line footers (e.g. BREAKING-CHANGE:)
// can be inspected by BumpByCommits.
func GetCommitMessagesSinceTag(repo *git.Repository, tagHash plumbing.Hash) ([]string, error) {
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("resolving HEAD: %w", err)
	}
	logIter, err := repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, err
	}

	var msgs []string
	err = logIter.ForEach(func(c *object.Commit) error {
		if c.Hash == tagHash {
			return storer.ErrStop
		}
		msg := strings.TrimSpace(c.Message)
		if msg != "" {
			msgs = append(msgs, msg)
		}
		return nil
	})
	return msgs, err
}

// hasBreakingChangeFooter returns true when any line in msg starts with
// "BREAKING CHANGE:" or "BREAKING-CHANGE:" (case-insensitive), which is the
// Conventional Commits footer token for breaking changes.
func hasBreakingChangeFooter(msg string) bool {
	for _, line := range strings.Split(msg, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "breaking change:") ||
			strings.HasPrefix(lower, "breaking-change:") {
			return true
		}
	}
	return false
}

// BumpByCommits applies the highest conventional-commit bump found across all
// messages.
//
// Bump rules (highest wins, short-circuits on Major):
//   - Subject line matches feat!: / fix!: (with optional scope) → Major
//   - Any line in full message starts with "BREAKING CHANGE:" or "BREAKING-CHANGE:" → Major
//   - Subject line matches feat: (with optional scope) → Minor
//   - Subject line matches fix: (with optional scope) → Patch
func BumpByCommits(v *SemVer, messages []string) {
	level := 0 // 0=none, 1=patch, 2=minor, 3=major
	for _, msg := range messages {
		subject := strings.SplitN(msg, "\n", 2)[0]
		if reBreakingType.MatchString(subject) || hasBreakingChangeFooter(msg) {
			level = 3
			break // can't go higher
		} else if reFeat.MatchString(subject) && level < 2 {
			level = 2
		} else if reFix.MatchString(subject) && level < 1 {
			level = 1
		}
	}
	switch level {
	case 3:
		v.BumpMajor()
	case 2:
		v.BumpMinor()
	case 1:
		v.BumpPatch()
	}
}

func (v *SemVer) BumpMajor() { v.Major++; v.Minor = 0; v.Patch = 0 }
func (v *SemVer) BumpMinor() { v.Minor++; v.Patch = 0 }
func (v *SemVer) BumpPatch() { v.Patch++ }

// BumpByCommitMessage is kept for backwards-compatibility and tests.
func BumpByCommitMessage(v *SemVer, msg string) {
	BumpByCommits(v, []string{msg})
}

// DetectMergeFromDevelop returns true if any message looks like a merge commit
// from the integration branch (e.g. "Merge branch 'develop' into 'main'").
// The branch name matched is controlled by GetDevelopBranch() (DEVELOP_BRANCH env var).
func DetectMergeFromDevelop(messages []string) bool {
	devBranch := strings.ToLower(GetDevelopBranch())
	for _, msg := range messages {
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "merge") && strings.Contains(lower, devBranch) {
			return true
		}
	}
	return false
}

// DetectMergeFromHotfix returns true if any message looks like a merge commit
// from a hotfix branch (e.g. "Merge branch 'hotfix/login-fix' into 'main'").
func DetectMergeFromHotfix(messages []string) bool {
	for _, msg := range messages {
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "merge") && strings.Contains(lower, "hotfix/") {
			return true
		}
	}
	return false
}

// DetectMergeFromReleaseBranch returns true if any message looks like a merge
// commit from a release branch (e.g. "Merge branch 'release/1.2' into 'main'").
func DetectMergeFromReleaseBranch(messages []string) bool {
	for _, msg := range messages {
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "merge") && strings.Contains(lower, "release/") {
			return true
		}
	}
	return false
}

// IsMainlineMerge reports whether a recognised source branch was merged into
// mainline, using the SOURCE_BRANCH env var (CI squash-merge path) first and
// falling back to commit-message pattern detection (regular merge path).
//
// Recognised source branches: develop (or DEVELOP_BRANCH), release/*, hotfix/*
//
// When this returns true the caller should:
//  1. Apply BumpByCommits to determine the bump level from commit messages.
//  2. Call CreateVersionTag to record the new version on mainline.
func IsMainlineMerge(messages []string) bool {
	src := strings.ToLower(strings.TrimSpace(GetSourceBranch()))
	if src != "" {
		switch {
		case src == strings.ToLower(GetDevelopBranch()),
			strings.HasPrefix(src, "release/"),
			strings.HasPrefix(src, "hotfix/"):
			return true
		}
	}
	return DetectMergeFromDevelop(messages) ||
		DetectMergeFromReleaseBranch(messages) ||
		DetectMergeFromHotfix(messages)
}

// IsMainlineSynced reports whether sourceBranchTip is reachable from the HEAD
// of developBranch. Use this to verify that a hotfix or release branch has been
// merged back into develop after being tagged on mainline.
//
//	sourceBranchTip = the commit hash at the tip of hotfix/* or release/*
//	                  before (or at) the merge into mainline.
//
// Returns true  → develop already contains the source-branch commits (in sync).
// Returns false → the source branch has not been merged to develop yet.
//
// Note: this check works for regular-merge workflows. For squash-merge
// workflows the hotfix/release commits are rewritten on main; sync must then
// be verified by other means (e.g. a VERSION file comparison).
func IsMainlineSynced(repo *git.Repository, developBranch string, sourceBranchTip plumbing.Hash) (bool, error) {
	ref, err := repo.Reference(plumbing.NewBranchReferenceName(developBranch), true)
	if err != nil {
		return false, fmt.Errorf("resolving branch %q: %w", developBranch, err)
	}
	logIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return false, err
	}
	found := false
	_ = logIter.ForEach(func(c *object.Commit) error {
		if c.Hash == sourceBranchTip {
			found = true
			return storer.ErrStop
		}
		return nil
	})
	return found, nil
}

// GetCurrentBranch returns the short branch name of HEAD.
// Returns an empty string on a detached HEAD.
func GetCurrentBranch(repo *git.Repository) string {
	head, err := repo.Head()
	if err != nil || !head.Name().IsBranch() {
		return ""
	}
	return head.Name().Short()
}

// CreateVersionTag creates a lightweight tag "vMAJOR.MINOR.PATCH" pointing to HEAD.
func CreateVersionTag(repo *git.Repository, v SemVer) error {
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("resolving HEAD: %w", err)
	}
	tagName := fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
	_, err = repo.CreateTag(tagName, head.Hash(), nil)
	return err
}

func ReadNextVersionFromYML() (string, error) {
	data, err := os.ReadFile("GitVersion.yml")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "next-version:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}
	return "", fmt.Errorf("next-version not found in GitVersion.yml")
}

// ReadVersionFromFile and WriteVersionToFile are kept for tests.
func ReadVersionFromFile(file string) (SemVer, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return SemVer{}, err
	}
	return ParseVersion(strings.TrimSpace(string(data)))
}

func WriteVersionToFile(file string, v SemVer) error {
	return os.WriteFile(file, []byte(fmt.Sprintf("%d.%d.%d\n", v.Major, v.Minor, v.Patch)), 0644)
}

func getGitInfo(repo *git.Repository) (branch, sha, shortSha, commitDate string, commitCount int) {
	head, err := repo.Head()
	if err != nil {
		return
	}

	// Branch name (empty string on detached HEAD).
	if head.Name().IsBranch() {
		branch = head.Name().Short()
	}

	sha = head.Hash().String()
	if len(sha) >= 7 {
		shortSha = sha[:7]
	} else {
		shortSha = sha
	}

	if commit, err := repo.CommitObject(head.Hash()); err == nil {
		commitDate = commit.Author.When.Format("2006-01-02")
	}

	// Count total commits reachable from HEAD.
	logIter, err := repo.Log(&git.LogOptions{From: head.Hash()})
	if err == nil {
		_ = logIter.ForEach(func(_ *object.Commit) error {
			commitCount++
			return nil
		})
	}
	return
}

func getUncommittedChanges(repo *git.Repository) int {
	wt, err := repo.Worktree()
	if err != nil {
		return 0
	}
	status, err := wt.Status()
	if err != nil {
		return 0
	}
	count := 0
	for _, s := range status {
		if s.Worktree != git.Unmodified || s.Staging != git.Unmodified {
			count++
		}
	}
	return count
}

func BuildVersionInfo(repo *git.Repository, v SemVer) VersionInfo {
	branch, sha, shortSha, commitDate, commitCount := getGitInfo(repo)
	mmp := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	return VersionInfo{
		Major:                     v.Major,
		Minor:                     v.Minor,
		Patch:                     v.Patch,
		BuildMetaData:             commitCount,
		FullBuildMetaData:         fmt.Sprintf("%d.Branch.%s.Sha.%s", commitCount, branch, sha),
		MajorMinorPatch:           mmp,
		SemVer:                    mmp,
		AssemblySemVer:            mmp + ".0",
		AssemblySemFileVer:        mmp + ".0",
		InformationalVersion:      fmt.Sprintf("%s+%d.Branch.%s.Sha.%s", mmp, commitCount, branch, sha),
		FullSemVer:                fmt.Sprintf("%s+%d", mmp, commitCount),
		BranchName:                branch,
		EscapedBranchName:         strings.ReplaceAll(branch, "/", "-"),
		Sha:                       sha,
		ShortSha:                  shortSha,
		VersionSourceSha:          sha,
		CommitsSinceVersionSource: commitCount,
		CommitDate:                commitDate,
		UncommittedChanges:        getUncommittedChanges(repo),
	}
}

func PrintJSON(info VersionInfo) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.Encode(info)
}
