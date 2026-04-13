# go-semver

A lightweight CLI tool that automates semantic versioning for Git repositories by reading [Conventional Commits](https://www.conventionalcommits.org/) and producing rich JSON metadata for CI/CD pipelines.

Version bumps are **always driven by commit messages** — never by branch name. The branch only controls where auto-tagging fires (mainline only).

## Install & Build

```bash
go build -o new-semver ./cmd/new-semver
```

## Usage

Run from the root of your Git repository:

```bash
./new-semver
```

Prints a JSON metadata object to stdout. No files are written or modified (version tags are created in the local git repo when a mainline merge is detected).

## How Version Resolution Works

```
Is there a semver git tag (e.g. v1.2.3)?
  ├── No  → read next-version: from GitVersion.yml and output as-is
  └── Yes → collect commits since that tag
              └── Any new commits?
                    ├── No  → output the tag version unchanged
                    └── Yes → on mainline + merge detected?
                                ├── Yes → bump by commits + create git tag
                                └── No  → bump by commits (no tag)
```

### Conventional Commits → Version Bump

Scopes are supported (`feat(auth):`, `fix(parser):`) and the highest bump across all commits wins.

| Commit Prefix | Example | Effect |
|---|---|---|
| `fix:` / `fix(scope):` | `fix: null pointer in auth` | PATCH bump |
| `feat:` / `feat(scope):` | `feat(api): add retry logic` | MINOR bump |
| `feat!:` / `fix!:` (with optional scope) | `feat!: drop v1 API` | MAJOR bump |
| `BREAKING CHANGE:` in footer | `BREAKING CHANGE: schema renamed` | MAJOR bump |
| `BREAKING-CHANGE:` in footer | `BREAKING-CHANGE: env vars changed` | MAJOR bump |
| Anything else (`chore:`, `docs:`, etc.) | `chore: update deps` | No bump |

> Matching is case-insensitive. When multiple commits are present, the highest bump wins.

## Branch Strategy

go-semver follows a GitFlow-inspired model. **Bump level is always commit-driven** — the branch name only controls whether a version tag is created.

```
Branch        Merges into          Bump          Auto-tag
────────────────────────────────────────────────────────────
main          —                    —             yes (on merge)
develop       main                 commit-driven on merge to main
release/*     main + develop       commit-driven on merge to main
hotfix/*      main + develop       commit-driven on merge to main
feature/*     develop              none          no
```

- **develop** — integration branch. Feature branches merge here; develop merges to main.
- **release/\*** — branched from develop to stabilise a release. Merges into main (triggers tag) and back into develop to keep them in sync.
- **hotfix/\*** — emergency fix branched from main. Merges into main (triggers tag) and back into develop.
- **feature/\*** — short-lived development branches. No tag, no bump; JSON output is informational only.

A version tag is created automatically when the tool runs on mainline and detects that a recognised source branch was merged (via `SOURCE_BRANCH` env var **or** "Merge branch '…'" commit message pattern).

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `MAINLINE_BRANCH` | `main` | Name of the production branch |
| `DEVELOP_BRANCH` | `develop` | Name of the integration branch (use `test` if your team calls it that) |
| `SOURCE_BRANCH` | _(empty)_ | Set by CI for squash merges to identify the source branch; falls back to commit-message detection when not set |

### Squash Merge Support

When using squash merges the resulting commit on main has no "Merge branch '…'" message. Set `SOURCE_BRANCH` in your CI pipeline to the name of the branch being merged (e.g. `develop`, `release/1.2`, `hotfix/fix-x`) so go-semver can detect the merge and create the version tag.

## Configuration

### GitVersion.yml

Used as the version source before the first git tag is created:

```yaml
next-version: 0.1.0       # output when no semver tag exists yet
mode: ContinuousDeployment
```

## JSON Output

```json
{
    "Major": 1,
    "Minor": 3,
    "Patch": 0,
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

## Docker

Pre-built images are published to GitHub Container Registry on every push to `main`:

```bash
docker pull ghcr.io/miraccan00/go-semver:latest
```

Run it against your local repository:

```bash
docker run --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  ghcr.io/miraccan00/go-semver:latest
```

> The container has `git` installed. Mount your repo root to `/workspace` so the tool can read `.git/` and `GitVersion.yml`.

---

## CI/CD Integration

### Quick Start (any CI)

Ensure the full git history is fetched (`--depth 0`) and run go-semver to capture the version:

```bash
VERSION_JSON=$(docker run --rm -v $(pwd):/workspace -w /workspace \
  ghcr.io/miraccan00/go-semver:latest)

APP_VERSION=$(echo "$VERSION_JSON" | jq -r '.MajorMinorPatch')
echo "Building version: $APP_VERSION"
```

No version file is written or committed back — go-semver manages versioning entirely through git tags.

---

### GitHub Actions

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: write   # required to push the version tag
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0          # required — go-semver reads full git log

      - name: Run go-semver
        id: semver
        env:
          SOURCE_BRANCH: ${{ github.head_ref }}   # set for squash-merge detection
        run: |
          VERSION_JSON=$(docker run --rm -v $(pwd):/workspace -w /workspace \
            -e SOURCE_BRANCH \
            ghcr.io/miraccan00/go-semver:latest)
          echo "version=$(echo $VERSION_JSON | jq -r '.MajorMinorPatch')" >> $GITHUB_OUTPUT

      - name: Use version
        run: |
          echo "App version: ${{ steps.semver.outputs.version }}"
          docker build -t myapp:${{ steps.semver.outputs.version }} .
```

> **Important:** Use `fetch-depth: 0` — the default shallow clone breaks `git log` and commit counting.

---

### GitLab CI

```yaml
variables:
  GIT_DEPTH: 0              # required — ensures full git history

semver:
  image: ghcr.io/miraccan00/go-semver:latest
  stage: .pre
  variables:
    SOURCE_BRANCH: $CI_MERGE_REQUEST_SOURCE_BRANCH_NAME   # squash-merge detection
  script:
    - VERSION_JSON=$(new-semver)
    - echo "APP_VERSION=$(echo $VERSION_JSON | jq -r '.MajorMinorPatch')" >> build.env
    - echo "SHORT_SHA=$(echo $VERSION_JSON | jq -r '.ShortSha')" >> build.env
  artifacts:
    reports:
      dotenv: build.env     # makes APP_VERSION available in downstream jobs

build:
  stage: build
  needs: [semver]
  script:
    - docker build -t myapp:$APP_VERSION .
```

---

### Jenkins (Declarative Pipeline)

```groovy
pipeline {
    agent any
    stages {
        stage('Version') {
            steps {
                script {
                    def versionJson = sh(
                        script: '''docker run --rm \
                            -v $(pwd):/workspace -w /workspace \
                            ghcr.io/miraccan00/go-semver:latest''',
                        returnStdout: true
                    ).trim()
                    env.APP_VERSION = sh(
                        script: "echo '${versionJson}' | jq -r '.MajorMinorPatch'",
                        returnStdout: true
                    ).trim()
                    echo "Building version: ${env.APP_VERSION}"
                }
            }
        }
        stage('Build') {
            steps {
                sh "docker build -t myapp:${env.APP_VERSION} ."
            }
        }
    }
}
```

---

### Available JSON Fields for CI

| Field | Example | Common Use |
|---|---|---|
| `MajorMinorPatch` | `1.3.0` | Docker image tag, artifact name |
| `FullSemVer` | `1.3.0+42` | Build metadata |
| `ShortSha` | `abc1234` | Traceability tag |
| `BranchName` | `main` | Conditional logic per branch |
| `EscapedBranchName` | `feature-login` | Safe for use in image tags |
| `CommitDate` | `2026-04-10` | Release notes |
| `UncommittedChanges` | `0` | Guard: fail build if dirty |

Fail the pipeline if there are uncommitted changes:

```bash
DIRTY=$(echo "$VERSION_JSON" | jq '.UncommittedChanges')
if [ "$DIRTY" -gt 0 ]; then
  echo "ERROR: $DIRTY uncommitted changes detected. Commit before building."
  exit 1
fi
```

---

## Known Limitations / Edge Cases

| Scenario | Current Behavior |
|---|---|
| Detached HEAD (typical CI checkout) | `BranchName` is empty string; no error raised |
| Squash merge without SOURCE_BRANCH | Falls back to commit-message detection; set `SOURCE_BRANCH` to guarantee tag creation |
| Pre-release versions (`1.0.0-rc.1`) | Fields exist in `VersionInfo` but are never populated |
| Multiple bump rules in one commit set | Highest bump wins: major → minor → patch |
| `git` binary not found | Git fields return zero values; no error |
| Empty repository (no commits yet) | GitVersion.yml fallback runs; git fields are empty strings |

## Testing

```bash
go test ./internal/semver/...
```

Tests cover: `ParseVersion`, `BumpByCommits` (Conventional Commits including scopes and `BREAKING-CHANGE` footer), `IsMainlineMerge`, `DetectMergeFrom*`, `GetMainlineBranch`, `GetDevelopBranch`, `GetSourceBranch`, `CreateVersionTag`, and full end-to-end branch-flow scenarios.

## Project Structure

```
cmd/new-semver/main.go                       # CLI entry point
internal/semver/semver.go                    # Core library
internal/semver/semver_test.go               # Unit tests
internal/semver/semver_scenario_test.go      # Integration / scenario tests
GitVersion.yml                               # Fallback version + branch config docs
ci-integration-example/                      # Ready-to-use CI pipeline templates
  github-actions.yml
  gitlab-ci.yml
  azure-pipelines.yml
```

## Contributing

Pull requests are welcome. When adding features, please cover edge cases with tests in `internal/semver/semver_test.go` and, for branch-flow behaviour, in `semver_scenario_test.go`.
