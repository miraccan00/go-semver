# CLAUDE.md — go-semver

## What this repository is

`go-semver` is a standalone CLI tool (`semver`) that computes a [Semantic Versioning](https://semver.org/) number for a Git repository by reading [Conventional Commits](https://www.conventionalcommits.org/) and outputs rich JSON metadata for use in CI/CD pipelines. It manages versioning entirely through **git tags** — no VERSION file is written or committed back.

---

## Repository layout

```
cmd/semver/main.go                       # CLI entry point
internal/semver/semver.go                # Core library (all logic lives here)
internal/semver/semver_test.go           # Unit tests
internal/semver/semver_scenario_test.go  # End-to-end branch-flow scenarios
GitVersion.yml                           # Fallback version config + branch docs
ci-integration-example/                  # Ready-to-use CI pipeline templates
  github-actions.yml
  gitlab-ci.yml
  azure-pipelines.yml
.github/workflows/ci.yml                 # PR validation: tests + build
.github/workflows/release.yml            # Merge to master: semver tag + Docker push
Dockerfile                               # Docker image definition
go.mod                                   # Module name: semver, Go 1.24.1
```

No `VERSION` file — that was removed. No `testdata/` directory — tests use `t.TempDir()`.

---

## Architecture: commit-driven versioning

**Rule**: Version bumps are **always** determined by Conventional Commit prefixes in commit messages. The source branch name **never** affects the bump magnitude — it only controls whether a version tag is created (mainline merges only).

```
Branch        Merges into          Bump              Auto-tag
────────────────────────────────────────────────────────────
main          —                    —                 yes (on merge)
develop       main                 commit-driven     on merge to main
release/*     main + develop       commit-driven     on merge to main
hotfix/*      main + develop       commit-driven     on merge to main
feature/*     develop              none              no
```

A version tag (`vX.Y.Z`) is created automatically when `semver` runs **on the mainline branch** and `IsMainlineMerge()` returns true.

---

## How the CLI works (`cmd/semver/main.go`)

```
1. OpenRepo()                         — open .git in cwd
2. GetLatestSemverTag(repo)
   ├── error → ReadNextVersionFromYML() → v = next-version (1.0.0)
   │           GetCurrentBranch == GetMainlineBranch() && IsMainlineMerge([])?
   │           ├── yes → CreateVersionTag(repo, v)   ← initial tag on first mainline merge
   │           └── no  → (no tag created)
   │           PrintJSON
   └── ok    → ParseTagVersion(tag)
3. GetCommitMessagesSinceTag(repo, tagHash)
   └── empty → PrintJSON (no new commits, current tag is correct)
4. GetCurrentBranch == GetMainlineBranch() && IsMainlineMerge(messages)?
   ├── yes → BumpByCommits(&v, messages)
   │         CreateVersionTag(repo, v)   ← auto-tag on mainline
   │         PrintJSON
   └── no  → BumpByCommits(&v, messages)
              PrintJSON
```

---

## Key functions in `internal/semver/semver.go`

### Environment variables

| Function | Env var | Default | Purpose |
|---|---|---|---|
| `GetMainlineBranch()` | `MAINLINE_BRANCH` | `"main"` | Production branch name |
| `GetDevelopBranch()` | `DEVELOP_BRANCH` | `"develop"` | Integration branch name (some teams use `"test"`) |
| `GetSourceBranch()` | `SOURCE_BRANCH` | `""` | Set by CI for squash merges so the tool knows what branch was merged |

### Conventional Commit parser

Package-level regexes (applied to first line / subject of each message):

```go
reBreakingType = regexp.MustCompile(`(?i)^(feat|fix)(\([^)]*\))?!:`)  // feat!: / fix!: / feat(scope)!:
reFeat         = regexp.MustCompile(`(?i)^feat(\([^)]*\))?:`)          // feat: / feat(scope):
reFix          = regexp.MustCompile(`(?i)^fix(\([^)]*\))?:`)           // fix: / fix(scope):
```

`hasBreakingChangeFooter(msg string) bool` — scans every line in a full commit message for lines that `HasPrefix` `"breaking change:"` or `"breaking-change:"` (case-insensitive). Uses `HasPrefix` not `Contains` to avoid false positives from body text like "no breaking change".

`BumpByCommits(v *SemVer, messages []string)` — iterates messages, applies highest bump (3=Major, 2=Minor, 1=Patch). Short-circuits on Major. Passes the **full message** to `hasBreakingChangeFooter` but only the first line (subject) to the regexes.

`GetCommitMessagesSinceTag` returns **full commit messages** (not just subject lines) so footer tokens are visible to `BumpByCommits`.

### Merge detection

```go
// Returns true when a recognised source branch was merged into mainline.
// Check order: SOURCE_BRANCH env var → commit message pattern matching.
// Recognised: develop (or DEVELOP_BRANCH value), release/*, hotfix/*
IsMainlineMerge(messages []string) bool

DetectMergeFromDevelop(messages []string) bool        // uses GetDevelopBranch() dynamically
DetectMergeFromHotfix(messages []string) bool          // looks for "hotfix/" in messages
DetectMergeFromReleaseBranch(messages []string) bool   // looks for "release/" in messages
```

### Sync verification

```go
// IsMainlineSynced checks whether sourceBranchTip is reachable from developBranch HEAD.
// Use after a hotfix or release merge to verify the branch was also merged back to develop.
// Works for regular merges only — squash merges rewrite commits, use other means then.
IsMainlineSynced(repo *git.Repository, developBranch string, sourceBranchTip plumbing.Hash) (bool, error)
```

### Tag management

```go
GetLatestSemverTag(repo) (string, plumbing.Hash, error)  // walks HEAD history, nearest semver tag
CreateVersionTag(repo, v SemVer) error                   // creates lightweight tag "vX.Y.Z" at HEAD
```

### Version bump methods on `SemVer`

```go
func (v *SemVer) BumpMajor() { v.Major++; v.Minor = 0; v.Patch = 0 }
func (v *SemVer) BumpMinor() { v.Minor++; v.Patch = 0 }
func (v *SemVer) BumpPatch() { v.Patch++ }
```

---

## Conventional Commits bump rules

| Commit prefix | Scope support | Effect |
|---|---|---|
| `fix:` / `fix(scope):` | yes | PATCH |
| `feat:` / `feat(scope):` | yes | MINOR |
| `feat!:` / `fix!:` / `feat(scope)!:` | yes | MAJOR |
| `BREAKING CHANGE:` in footer | — | MAJOR |
| `BREAKING-CHANGE:` in footer (hyphen) | — | MAJOR |
| `chore:`, `docs:`, anything else | — | no bump |

Matching is case-insensitive. Highest bump across all commits in the set wins.

---

## Squash-merge support

When teams use squash merges, the resulting commit on `main` has **no** "Merge branch '…'" message. The CI must set `SOURCE_BRANCH` to the name of the branch being merged (e.g. `develop`, `release/1.2`, `hotfix/fix-x`).

- GitLab: `SOURCE_BRANCH: $CI_MERGE_REQUEST_SOURCE_BRANCH_NAME`
- GitHub Actions: `SOURCE_BRANCH: ${{ github.head_ref }}`
- Azure Pipelines: `-e SOURCE_BRANCH=$(System.PullRequest.SourceBranch)`

Without `SOURCE_BRANCH`, the tool falls back to commit-message pattern detection (works for regular merges).

---

## JSON output fields

```json
{
  "Major": 1, "Minor": 3, "Patch": 0,
  "MajorMinorPatch": "1.3.0",
  "SemVer": "1.3.0",
  "FullSemVer": "1.3.0+42",
  "InformationalVersion": "1.3.0+42.Branch.main.Sha.abc1234...",
  "BranchName": "main",
  "EscapedBranchName": "main",
  "Sha": "abc1234...",
  "ShortSha": "abc1234",
  "CommitsSinceVersionSource": 42,
  "CommitDate": "2026-04-10",
  "UncommittedChanges": 0,
  "BuildMetaData": 42,
  "AssemblySemVer": "1.3.0.0",
  "AssemblySemFileVer": "1.3.0.0"
}
```

`EscapedBranchName` replaces `/` with `-`, safe for Docker image tags.

---

## GitVersion.yml

Serves two purposes:
1. **Fallback version** — `next-version: 1.0.0` is used when no semver tag exists yet. On the first mainline merge (`SOURCE_BRANCH` detected), the tool creates `v1.0.0` automatically so subsequent runs have a baseline.
2. **Documentation** — contains the branch strategy table and env var docs as comments. It is **not** a runtime config file; only `next-version:` is read by the code (`ReadNextVersionFromYML()`).

---

## Docker

Pre-built image: `ghcr.io/miraccan00/go-semver:latest`

Run against a local repo:
```bash
docker run --rm -v $(pwd):/workspace -w /workspace ghcr.io/miraccan00/go-semver:latest
```

Mount your repo root to `/workspace`; the container has `git` installed.

The tool's own Docker image is built and pushed by [`.github/workflows/release.yml`](.github/workflows/release.yml) on every merge to master. There is no separate docker-push workflow.

---

## Tests

```bash
go test ./...                    # run all tests
go test ./internal/semver/...    # library tests only
go build ./cmd/semver/       # verify binary compiles
```

### Test pattern: Arrange-Act-Assert (AAA)

All tests in this repository follow the **Arrange-Act-Assert** structure. Each section is marked with a comment block:

```go
// Arrange  — set up inputs, env vars, repo state
// Act      — call the single function under test
// Assert   — verify the result
```

**Rules:**
- One `// Act` block per test (or per sub-scenario in multi-step scenario tests).
- Keep arrange and assert clearly separated from act — do not interleave assertions inside the act block.
- Table-driven tests use `t.Run(c.name, ...)` so every case is individually named and independently re-runnable. The table definition is the shared arrange; per-row setup (e.g. `t.Setenv`) goes inside `t.Run`.
- Multi-step scenario tests (Scenario 4, 5, 7) repeat the act/assert pair with a short inline comment that names the phase, e.g.:

```go
// Act: check sync before merging hotfix back to develop
synced, err := IsMainlineSynced(repo, "develop", hotfixTip)

// Assert: develop is out of sync
if synced { t.Error("...") }

// Act: regular merge hotfix → develop
addMergeCommit(...)

// Assert: develop is now in sync
synced, err = IsMainlineSynced(repo, "develop", hotfixTip)
```

### Test files

**`semver_test.go`** — unit tests:
- `TestReadWriteVersionFromFile` — two subtests: "reads version correctly" / "persists bumped patch version"; uses `t.TempDir()`
- `TestParseVersion`
- `TestBumpByCommitMessage` — 6 CC cases, each in a named `t.Run`
- `TestGetMainlineBranch`, `TestGetDevelopBranch`, `TestGetSourceBranch` — env var table tests with named subtests
- `TestIsMainlineMerge_SourceBranchEnvVar` — SOURCE_BRANCH detection, named subtests
- `TestIsMainlineMerge_EnvVarTakesPriorityOverMessage`
- `TestIsMainlineMerge_FallsBackToMessages` — named subtests
- `TestDetectMergeFromDevelop`, `TestDetectMergeFromDevelop_CustomBranchName` — named subtests
- `TestDetectMergeFromHotfix`, `TestDetectMergeFromReleaseBranch` — named subtests
- `TestIsMainlineMerge` — named subtests (desc field used as subtest name)
- `TestBumpByCommits_Scope` — `feat(auth):`, `fix(parser):`, `feat(ui)!:`, named subtests
- `TestBumpByCommits_BreakingChangeFooter` — multi-line messages with footer tokens, named subtests
- `TestCreateVersionTag`, `TestCreateVersionTagNoDuplicate`

**`semver_scenario_test.go`** — integration scenarios using real in-process git repos (`git.PlainInit` in a temp dir). Each scenario is independent:
- Scenario 1a: Fresh repo (no tags) → `GetLatestSemverTag` returns error (fallback signal)
- Scenario 1b: Fresh repo + `SOURCE_BRANCH=develop` on master → `IsMainlineMerge` true → initial `v1.0.0` tag created from `next-version`
- Scenario 2: Features squash-merged to develop → Minor bump, no auto-tag
- Scenario 3: Squash develop→main, `feat!:` → Major (v2.0.0), tag created
- Scenario 4: Hotfix `fix:` → Patch (v1.0.1), `IsMainlineSynced` before/after sync (two act/assert cycles)
- Scenario 5: Release `feat:` → Minor (v1.1.0), `IsMainlineSynced` before/after sync (two act/assert cycles)
- Scenario 6: `MAINLINE_BRANCH=master` custom mainline, `feat!:` → Major (v4.0.0)
- Scenario 7: `DEVELOP_BRANCH=test` — SOURCE_BRANCH path, message path, and `IsMainlineSynced` all respect the custom name

Squash merges are simulated by: checkout target branch → add CC-typed commit → set `SOURCE_BRANCH`.
Regular merges are simulated via `CommitOptions.Parents` with second parent = source tip (makes source commit reachable from target branch history, which is what `IsMainlineSynced` checks).

---

## Module & dependencies

Module name: `semver` (in `go.mod`)

Key dependency: `github.com/go-git/go-git/v5` — pure-Go git implementation used for all git operations (no `git` binary required at runtime; it is included in the Docker image for the few places that need it).

---

## Common tasks

**Add a new env var** — follow the `GetDevelopBranch()` pattern: read env, return default. Update `GitVersion.yml` comments, `README.md`, `TR-README.md`, and add a `TestGetXxx` unit test.

**Add a new CC type** — add a regex at package level, update `BumpByCommits`, add test cases in `TestBumpByCommits_*`.

**Add a new branch type** — add a `DetectMergeFrom*` function, call it in `IsMainlineMerge`, update `GitVersion.yml`, `README.md`, `TR-README.md`, and add a scenario test.

**Change bump logic** — all bump logic is in `BumpByCommits` in `semver.go`. The CC regexes are package-level vars.

**Debug version output** — run `./semver` from the repo root; it prints JSON to stdout. Set `SOURCE_BRANCH`, `MAINLINE_BRANCH`, or `DEVELOP_BRANCH` env vars as needed to simulate CI conditions.

---

## GitHub repository setup (one-time)

### Branch protection rules

Configure via **Settings → Branches → Add rule** in the GitHub UI, or with `gh`:

```bash
# Protect master: require PR, require CI to pass, no direct push
gh api repos/{owner}/go-semver/branches/master/protection \
  --method PUT \
  --input - <<'EOF'
{
  "required_status_checks": {
    "strict": true,
    "contexts": ["test"]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": {
    "required_approving_review_count": 1,
    "dismiss_stale_reviews": true
  },
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false
}
EOF

# Protect develop: require PR, require CI to pass
gh api repos/{owner}/go-semver/branches/develop/protection \
  --method PUT \
  --input - <<'EOF'
{
  "required_status_checks": {
    "strict": false,
    "contexts": ["test"]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": {
    "required_approving_review_count": 1
  },
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false
}
EOF
```

### GitFlow — branching rules

```
master     ← PRs only from: develop, release/*, hotfix/*
develop    ← PRs only from: feature/*
feature/*  ← branch from develop; delete after merge
release/*  ← branch from develop; merge to master AND back to develop
hotfix/*   ← branch from master; merge to master AND back to develop
```

**Workflow triggered on PR merge to master** (`release.yml`):
1. `SOURCE_BRANCH` is read from `github.event.pull_request.head.ref` — no commit-message parsing needed, works with both merge and squash strategies.
2. Semver container (`ghcr.io/miraccan00/go-semver:latest`) runs in the workspace with `MAINLINE_BRANCH=master` and the detected `SOURCE_BRANCH`.
3. If `IsMainlineMerge` returns true, the tool bumps the version, creates the tag in `.git`, and the workflow pushes it.
4. Docker image is built and pushed as `ghcr.io/miraccan00/go-semver:<version>` and `:latest`.

---

## Important design decisions (do not revert)
- `hasBreakingChangeFooter` uses `HasPrefix` (not `Contains`) to avoid matching body prose like "no breaking change" — this was a deliberate fix for a test regression.
- `GetCommitMessagesSinceTag` returns **full** messages (not just subject lines) — required so footer tokens are detectable.
- `IsMainlineMerge` is purely a detection function; it does **not** apply a bump. Bump is always `BumpByCommits`.
- There is no `BumpForMainlineMerge` function — it was removed when source-branch-based bumping was replaced with commit-driven bumping.
- `release.yml` uses `pull_request: types: [closed]` (not `push: branches: [master]`) so that `github.event.pull_request.head.ref` is available for `SOURCE_BRANCH` detection — avoids fragile commit-message parsing for GitHub-style merge commits ("Merge pull request #N from owner/branch").
