package semver

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestReadWriteVersionFromFile(t *testing.T) {
	t.Run("reads version correctly", func(t *testing.T) {
		// Arrange
		file := filepath.Join(t.TempDir(), "VERSION")
		_ = os.WriteFile(file, []byte("1.2.3\n"), 0644)

		// Act
		v, err := ReadVersionFromFile(file)

		// Assert
		if err != nil {
			t.Fatalf("ReadVersionFromFile failed: %v", err)
		}
		if v.Major != 1 || v.Minor != 2 || v.Patch != 3 {
			t.Errorf("unexpected version: %+v", v)
		}
	})

	t.Run("persists bumped patch version", func(t *testing.T) {
		// Arrange
		file := filepath.Join(t.TempDir(), "VERSION")
		_ = os.WriteFile(file, []byte("1.2.3\n"), 0644)
		v, _ := ReadVersionFromFile(file)
		v.BumpPatch()

		// Act
		err := WriteVersionToFile(file, v)

		// Assert
		if err != nil {
			t.Fatalf("WriteVersionToFile failed: %v", err)
		}
		v2, _ := ReadVersionFromFile(file)
		if v2.Patch != 4 {
			t.Errorf("patch bump failed: got %d", v2.Patch)
		}
	})
}

func TestParseVersion(t *testing.T) {
	// Arrange
	input := "2.5.9"

	// Act
	v, err := ParseVersion(input)

	// Assert
	if err != nil {
		t.Fatalf("ParseVersion failed: %v", err)
	}
	if v.Major != 2 || v.Minor != 5 || v.Patch != 9 {
		t.Errorf("unexpected version: %+v", v)
	}
}

func TestBumpByCommitMessage(t *testing.T) {
	cases := []struct {
		name   string
		msg    string
		start  SemVer
		expect SemVer
	}{
		{"fix bumps patch", "fix: something", SemVer{1, 2, 3}, SemVer{1, 2, 4}},
		{"feat bumps minor", "feat: new", SemVer{1, 2, 3}, SemVer{1, 3, 0}},
		{"feat! bumps major", "feat!: breaking", SemVer{1, 2, 3}, SemVer{2, 0, 0}},
		{"fix! bumps major", "fix!: breaking", SemVer{1, 2, 3}, SemVer{2, 0, 0}},
		{"BREAKING CHANGE bumps major", "BREAKING CHANGE: api", SemVer{1, 2, 3}, SemVer{2, 0, 0}},
		{"chore does not bump", "chore: nothing", SemVer{1, 2, 3}, SemVer{1, 2, 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			v := c.start

			// Act
			BumpByCommitMessage(&v, c.msg)

			// Assert
			if v != c.expect {
				t.Errorf("got %+v, want %+v", v, c.expect)
			}
		})
	}
}

func TestGetMainlineBranch(t *testing.T) {
	cases := []struct {
		name   string
		envVal string
		expect string
	}{
		{"defaults to main when unset", "", "main"},
		{"respects custom value master", "master", "master"},
		{"respects explicit main", "main", "main"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			t.Setenv("MAINLINE_BRANCH", c.envVal)

			// Act
			got := GetMainlineBranch()

			// Assert
			if got != c.expect {
				t.Errorf("MAINLINE_BRANCH=%q: got %q, want %q", c.envVal, got, c.expect)
			}
		})
	}
}

func TestGetDevelopBranch(t *testing.T) {
	cases := []struct {
		name   string
		envVal string
		expect string
	}{
		{"defaults to develop when unset", "", "develop"},
		{"respects custom value test", "test", "test"},
		{"respects explicit develop", "develop", "develop"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			t.Setenv("DEVELOP_BRANCH", c.envVal)

			// Act
			got := GetDevelopBranch()

			// Assert
			if got != c.expect {
				t.Errorf("DEVELOP_BRANCH=%q: got %q, want %q", c.envVal, got, c.expect)
			}
		})
	}
}

func TestGetSourceBranch(t *testing.T) {
	t.Run("not set returns empty string", func(t *testing.T) {
		// Arrange
		t.Setenv("SOURCE_BRANCH", "")

		// Act
		got := GetSourceBranch()

		// Assert
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("returns env var value", func(t *testing.T) {
		// Arrange
		t.Setenv("SOURCE_BRANCH", "develop")

		// Act
		got := GetSourceBranch()

		// Assert
		if got != "develop" {
			t.Errorf("expected %q, got %q", "develop", got)
		}
	})
}

func TestIsMainlineMerge_SourceBranchEnvVar(t *testing.T) {
	cases := []struct {
		name   string
		src    string
		expect bool
	}{
		{"develop is recognised", "develop", true},
		{"release branch is recognised", "release/2.0", true},
		{"hotfix branch is recognised", "hotfix/login", true},
		{"feature branch is not a mainline merge", "feature/new-ui", false},
		{"empty SOURCE_BRANCH is not a mainline merge", "", false},
	}
	// Messages deliberately contain no merge pattern to isolate SOURCE_BRANCH detection.
	msgs := []string{"chore: squash merge"}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			t.Setenv("SOURCE_BRANCH", c.src)

			// Act
			got := IsMainlineMerge(msgs)

			// Assert
			if got != c.expect {
				t.Errorf("SOURCE_BRANCH=%q: got %v, want %v", c.src, got, c.expect)
			}
		})
	}
}

func TestIsMainlineMerge_EnvVarTakesPriorityOverMessage(t *testing.T) {
	// Arrange
	t.Setenv("SOURCE_BRANCH", "develop")
	msgs := []string{"Merge branch 'hotfix/login-fix'"}

	// Act
	got := IsMainlineMerge(msgs)

	// Assert
	if !got {
		t.Error("expected IsMainlineMerge=true when SOURCE_BRANCH=develop overrides hotfix message")
	}
}

func TestIsMainlineMerge_FallsBackToMessages(t *testing.T) {
	cases := []struct {
		name   string
		msgs   []string
		expect bool
	}{
		{"develop merge message detected", []string{"Merge branch 'develop' into 'main'"}, true},
		{"release merge message detected", []string{"Merge branch 'release/1.2' into 'main'"}, true},
		{"hotfix merge message detected", []string{"Merge branch 'hotfix/critical' into 'main'"}, true},
		{"conventional commit only — no merge", []string{"feat: new feature"}, false},
		{"empty messages — no merge", []string{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			t.Setenv("SOURCE_BRANCH", "")

			// Act
			got := IsMainlineMerge(c.msgs)

			// Assert
			if got != c.expect {
				t.Errorf("got %v, want %v", got, c.expect)
			}
		})
	}
}

func TestDetectMergeFromDevelop(t *testing.T) {
	cases := []struct {
		name   string
		msgs   []string
		expect bool
	}{
		{"bare develop merge message", []string{"Merge branch 'develop'"}, true},
		{"develop merge into master", []string{"Merge branch 'develop' into 'master'"}, true},
		{"remote-tracking develop merge", []string{"Merge remote-tracking branch 'origin/develop'"}, true},
		{"only conventional commits — not a merge", []string{"feat: new feature", "fix: bug"}, false},
		{"chore message — not a merge", []string{"chore: release"}, false},
		{"empty messages", []string{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange — messages already in c.msgs; no extra setup needed

			// Act
			got := DetectMergeFromDevelop(c.msgs)

			// Assert
			if got != c.expect {
				t.Errorf("got %v, want %v", got, c.expect)
			}
		})
	}
}

func TestDetectMergeFromDevelop_CustomBranchName(t *testing.T) {
	cases := []struct {
		name   string
		msgs   []string
		expect bool
	}{
		{"merge of custom branch 'test' is detected", []string{"Merge branch 'test' into 'main'"}, true},
		{"'develop' no longer matches when DEVELOP_BRANCH=test", []string{"Merge branch 'develop' into 'main'"}, false},
		{"conventional commit — not a merge", []string{"feat: feature"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			t.Setenv("DEVELOP_BRANCH", "test")

			// Act
			got := DetectMergeFromDevelop(c.msgs)

			// Assert
			if got != c.expect {
				t.Errorf("got %v, want %v", got, c.expect)
			}
		})
	}
}

func TestDetectMergeFromHotfix(t *testing.T) {
	cases := []struct {
		name   string
		msgs   []string
		expect bool
	}{
		{"bare hotfix merge message", []string{"Merge branch 'hotfix/critical-fix'"}, true},
		{"hotfix merge into main", []string{"Merge branch 'hotfix/null-pointer' into 'main'"}, true},
		{"feature commit — not a hotfix merge", []string{"feat: new feature"}, false},
		{"develop merge — not a hotfix merge", []string{"Merge branch 'develop'"}, false},
		{"empty messages", []string{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange — messages already in c.msgs; no extra setup needed

			// Act
			got := DetectMergeFromHotfix(c.msgs)

			// Assert
			if got != c.expect {
				t.Errorf("got %v, want %v", got, c.expect)
			}
		})
	}
}

func TestDetectMergeFromReleaseBranch(t *testing.T) {
	cases := []struct {
		name   string
		msgs   []string
		expect bool
	}{
		{"bare release merge message", []string{"Merge branch 'release/1.2'"}, true},
		{"release merge into main", []string{"Merge branch 'release/2024-q1' into 'main'"}, true},
		{"feature commit — not a release merge", []string{"feat: feature"}, false},
		{"develop merge — not a release merge", []string{"Merge branch 'develop'"}, false},
		{"empty messages", []string{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange — messages already in c.msgs; no extra setup needed

			// Act
			got := DetectMergeFromReleaseBranch(c.msgs)

			// Assert
			if got != c.expect {
				t.Errorf("got %v, want %v", got, c.expect)
			}
		})
	}
}

func TestIsMainlineMerge(t *testing.T) {
	cases := []struct {
		desc   string
		msgs   []string
		expect bool
	}{
		{
			"develop merge commit message",
			[]string{"Merge branch 'develop' into 'main'"},
			true,
		},
		{
			"release merge commit message",
			[]string{"Merge branch 'release/2.0' into 'main'"},
			true,
		},
		{
			"hotfix merge commit message",
			[]string{"Merge branch 'hotfix/login-fix' into 'main'"},
			true,
		},
		{
			"conventional commits only — no merge detected",
			[]string{"feat: add login", "fix: null pointer"},
			false,
		},
		{
			"empty messages",
			[]string{},
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			// Arrange
			t.Setenv("SOURCE_BRANCH", "")

			// Act
			got := IsMainlineMerge(c.msgs)

			// Assert
			if got != c.expect {
				t.Errorf("got %v, want %v", got, c.expect)
			}
		})
	}
}

func TestBumpByCommits_Scope(t *testing.T) {
	cases := []struct {
		name   string
		msg    string
		start  SemVer
		expect SemVer
	}{
		{"feat with scope bumps minor", "feat(auth): add OAuth", SemVer{1, 0, 0}, SemVer{1, 1, 0}},
		{"fix with scope bumps patch", "fix(parser): handle empty input", SemVer{1, 2, 3}, SemVer{1, 2, 4}},
		{"feat with scope and ! bumps major", "feat(ui)!: redesign dashboard", SemVer{1, 2, 3}, SemVer{2, 0, 0}},
		{"fix with scope and ! bumps major", "fix(api)!: remove deprecated endpoint", SemVer{1, 2, 3}, SemVer{2, 0, 0}},
		{"chore with scope does not bump", "chore(deps): update go modules", SemVer{1, 0, 0}, SemVer{1, 0, 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			v := c.start

			// Act
			BumpByCommits(&v, []string{c.msg})

			// Assert
			if v != c.expect {
				t.Errorf("got %+v, want %+v", v, c.expect)
			}
		})
	}
}

func TestBumpByCommits_BreakingChangeFooter(t *testing.T) {
	cases := []struct {
		desc   string
		msg    string
		start  SemVer
		expect SemVer
	}{
		{
			"BREAKING CHANGE in footer",
			"feat: allow provided config object to extend other configs\n\nBREAKING CHANGE: `extends` key in config file is now used for extending other configs",
			SemVer{1, 2, 3}, SemVer{2, 0, 0},
		},
		{
			"BREAKING-CHANGE with hyphen in footer",
			"fix: prevent racing of requests\n\nBREAKING-CHANGE: environment variables now take precedence over config files",
			SemVer{1, 2, 3}, SemVer{2, 0, 0},
		},
		{
			"multi-paragraph body with BREAKING CHANGE footer",
			"feat: add new endpoint\n\nSome longer description.\n\nBREAKING CHANGE: old /v1 endpoint removed",
			SemVer{2, 5, 1}, SemVer{3, 0, 0},
		},
		{
			"feat with no footer — Minor only",
			"feat: add new feature\n\nLonger description with no breaking change.",
			SemVer{1, 0, 0}, SemVer{1, 1, 0},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			// Arrange
			v := c.start

			// Act
			BumpByCommits(&v, []string{c.msg})

			// Assert
			if v != c.expect {
				t.Errorf("got %+v, want %+v", v, c.expect)
			}
		})
	}
}

func TestCreateVersionTag(t *testing.T) {
	// Arrange
	repo, _ := initTempRepo(t)
	v := SemVer{2, 0, 0}

	// Act
	err := CreateVersionTag(repo, v)

	// Assert
	if err != nil {
		t.Fatalf("CreateVersionTag: %v", err)
	}
	tagName, _, err := GetLatestSemverTag(repo)
	if err != nil {
		t.Fatalf("GetLatestSemverTag: %v", err)
	}
	if tagName != "v2.0.0" {
		t.Errorf("expected tag v2.0.0, got %q", tagName)
	}
}

func TestCreateVersionTagNoDuplicate(t *testing.T) {
	// Arrange
	repo, _ := initTempRepo(t)
	v := SemVer{1, 0, 0}
	if err := CreateVersionTag(repo, v); err != nil {
		t.Fatalf("first CreateVersionTag: %v", err)
	}

	// Act
	err := CreateVersionTag(repo, v)

	// Assert
	if err == nil {
		t.Error("expected error when creating duplicate tag, got nil")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// initTempRepo creates a bare-minimum git repo in a temp dir with one commit.
func initTempRepo(t *testing.T) (*git.Repository, string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	readmePath := filepath.Join(dir, "README")
	if err := os.WriteFile(readmePath, []byte("init"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := wt.Add("README"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return repo, dir
}
