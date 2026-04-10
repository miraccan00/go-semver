package semver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

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

func CheckGitRepoExists() bool {
	info, err := os.Stat(".git")
	return err == nil && info.IsDir()
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

// GetLatestSemverTag returns the most recent git tag that looks like a semver
// (with or without a "v" prefix). Returns an error when no such tag exists.
func GetLatestSemverTag() (string, error) {
	// v-prefixed: v1.2.3
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*.[0-9]*.[0-9]*")
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	// bare: 1.2.3
	cmd = exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "[0-9]*.[0-9]*.[0-9]*")
	out, err = cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	return "", fmt.Errorf("no semver tag found")
}

// GetCommitMessagesSinceTag returns the subject line of every commit reachable
// from HEAD but not from tag. Pass an empty tag to get all commits.
func GetCommitMessagesSinceTag(tag string) ([]string, error) {
	var args []string
	if tag == "" {
		args = []string{"log", "--pretty=%s"}
	} else {
		args = []string{"log", tag + "..HEAD", "--pretty=%s"}
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, err
	}
	var msgs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			msgs = append(msgs, line)
		}
	}
	return msgs, nil
}

// BumpByCommits applies the highest conventional-commit bump found across all
// messages: BREAKING CHANGE / feat! / fix! → major, feat → minor, fix → patch.
func BumpByCommits(v *SemVer, messages []string) {
	level := 0 // 0=none, 1=patch, 2=minor, 3=major
	for _, msg := range messages {
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "breaking change") ||
			strings.HasPrefix(lower, "feat!:") ||
			strings.HasPrefix(lower, "fix!:") {
			level = 3
			break // can't go higher
		} else if strings.HasPrefix(lower, "feat:") && level < 2 {
			level = 2
		} else if strings.HasPrefix(lower, "fix:") && level < 1 {
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

func ReadNextVersionFromYML() (string, error) {
	data, err := ioutil.ReadFile("GitVersion.yml")
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
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return SemVer{}, err
	}
	return ParseVersion(strings.TrimSpace(string(data)))
}

func WriteVersionToFile(file string, v SemVer) error {
	return ioutil.WriteFile(file, []byte(fmt.Sprintf("%d.%d.%d\n", v.Major, v.Minor, v.Patch)), 0644)
}

func getCurrentGitBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func getGitInfo() (branch, sha, shortSha, commitDate string, commitsSince int) {
	branch = getCurrentGitBranch()
	if branch != "" {
		if shaBytes, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
			sha = strings.TrimSpace(string(shaBytes))
			if len(sha) > 7 {
				shortSha = sha[:7]
			} else {
				shortSha = sha
			}
		}
		if dateBytes, err := exec.Command("git", "log", "-1", "--format=%cd", "--date=short").Output(); err == nil {
			commitDate = strings.TrimSpace(string(dateBytes))
		}
		if countBytes, err := exec.Command("git", "rev-list", "--count", "HEAD").Output(); err == nil {
			commitsSince, _ = strconv.Atoi(strings.TrimSpace(string(countBytes)))
		}
	}
	return
}

func getUncommittedChanges() int {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0
	}
	return len(lines)
}

func BuildVersionInfo(v SemVer) VersionInfo {
	branch, sha, shortSha, commitDate, commitsSince := getGitInfo()
	mmp := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	return VersionInfo{
		Major:                     v.Major,
		Minor:                     v.Minor,
		Patch:                     v.Patch,
		BuildMetaData:             commitsSince,
		FullBuildMetaData:         fmt.Sprintf("%d.Branch.%s.Sha.%s", commitsSince, branch, sha),
		MajorMinorPatch:           mmp,
		SemVer:                    mmp,
		AssemblySemVer:            mmp + ".0",
		AssemblySemFileVer:        mmp + ".0",
		InformationalVersion:      fmt.Sprintf("%s+%d.Branch.%s.Sha.%s", mmp, commitsSince, branch, sha),
		FullSemVer:                fmt.Sprintf("%s+%d", mmp, commitsSince),
		BranchName:                branch,
		EscapedBranchName:         strings.ReplaceAll(branch, "/", "-"),
		Sha:                       sha,
		ShortSha:                  shortSha,
		VersionSourceSha:          sha,
		CommitsSinceVersionSource: commitsSince,
		CommitDate:                commitDate,
		UncommittedChanges:        getUncommittedChanges(),
	}
}

func PrintJSON(info VersionInfo) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.Encode(info)
}
