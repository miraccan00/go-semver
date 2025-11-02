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

const versionFile = "VERSION"

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

func ReadVersionFromFile(file string) (SemVer, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return SemVer{}, err
	}
	parts := strings.Split(strings.TrimSpace(string(data)), ".")
	if len(parts) != 3 {
		return SemVer{}, fmt.Errorf("invalid version format")
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])
	return SemVer{major, minor, patch}, nil
}

func ReadVersion() (SemVer, error) {
	return ReadVersionFromFile(versionFile)
}

func WriteVersionToFile(file string, v SemVer) error {
	versionStr := fmt.Sprintf("%d.%d.%d\n", v.Major, v.Minor, v.Patch)
	return ioutil.WriteFile(file, []byte(versionStr), 0644)
}

func WriteVersion(v SemVer) error {
	return WriteVersionToFile(versionFile, v)
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

func GetLastCommitMessage() (string, error) {
	cmd := exec.Command("git", "log", "-1", "--pretty=%B")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func BumpByCommitMessage(v *SemVer, msg string) {
	msg = strings.ToLower(msg)
	if strings.Contains(msg, "breaking change") || strings.HasPrefix(msg, "feat!:") || strings.HasPrefix(msg, "fix!:") {
		v.BumpMajor()
	} else if strings.HasPrefix(msg, "feat:") {
		v.BumpMinor()
	} else if strings.HasPrefix(msg, "fix:") {
		v.BumpPatch()
	} // else: no bump
}

func (v *SemVer) BumpMajor() {
	v.Major++
	v.Minor = 0
	v.Patch = 0
}

func (v *SemVer) BumpMinor() {
	v.Minor++
	v.Patch = 0
}

func (v *SemVer) BumpPatch() {
	v.Patch++
}

func ReadNextVersionFromYML() (string, error) {
	data, err := ioutil.ReadFile("GitVersion.yml")
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "next-version:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}
	return "", fmt.Errorf("next-version not found in GitVersion.yml")
}

func getCurrentGitBranch() string {
       cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
       out, err := cmd.Output()
       if err != nil {
	       return ""
       }
       return strings.TrimSpace(string(out))
}

func getGitInfo() (branch, sha, shortSha, commitDate string, commitsSince int) {
       branch = getCurrentGitBranch()
       sha = ""
       shortSha = ""
       commitDate = ""
       commitsSince = 0
       if branch != "" {
	       shaBytes, err := exec.Command("git", "rev-parse", "HEAD").Output()
	       if err == nil {
		       sha = strings.TrimSpace(string(shaBytes))
		       if len(sha) > 7 {
			       shortSha = sha[:7]
		       } else {
			       shortSha = sha
		       }
	       }
	       dateBytes, err := exec.Command("git", "log", "-1", "--format=%cd", "--date=short").Output()
	       if err == nil {
		       commitDate = strings.TrimSpace(string(dateBytes))
	       }
	       countBytes, err := exec.Command("git", "rev-list", "--count", "HEAD").Output()
	       if err == nil {
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
       uncommitted := getUncommittedChanges()
       majorMinorPatch := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
       semver := majorMinorPatch
       info := VersionInfo{
	       Major:                     v.Major,
	       Minor:                     v.Minor,
	       Patch:                     v.Patch,
	       PreReleaseTag:             "",
	       PreReleaseTagWithDash:     "",
	       PreReleaseLabel:           "",
	       PreReleaseLabelWithDash:   "",
	       PreReleaseNumber:          0,
	       WeightedPreReleaseNumber:  0,
	       BuildMetaData:             commitsSince,
	       FullBuildMetaData:         fmt.Sprintf("%d.Branch.%s.Sha.%s", commitsSince, branch, sha),
	       MajorMinorPatch:           majorMinorPatch,
	       SemVer:                    semver,
	       AssemblySemVer:            majorMinorPatch + ".0",
	       AssemblySemFileVer:        majorMinorPatch + ".0",
	       InformationalVersion:      fmt.Sprintf("%s+%d.Branch.%s.Sha.%s", semver, commitsSince, branch, sha),
	       FullSemVer:                fmt.Sprintf("%s+%d", semver, commitsSince),
	       BranchName:                branch,
	       EscapedBranchName:         strings.ReplaceAll(branch, "/", "-"),
	       Sha:                       sha,
	       ShortSha:                  shortSha,
	       VersionSourceSha:          sha,
	       CommitsSinceVersionSource: commitsSince,
	       CommitDate:                commitDate,
	       UncommittedChanges:        uncommitted,
       }
       return info
}

func PrintJSON(info VersionInfo) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.Encode(info)
}
