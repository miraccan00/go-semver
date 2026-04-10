# go-semver

A lightweight CLI tool that automates semantic versioning for Git repositories by reading [Conventional Commits](https://www.conventionalcommits.org/) and producing rich JSON metadata for CI/CD pipelines.

## Install & Build

```bash
go build -o new-semver ./cmd/new-semver
```

## Usage

Run from the root of your Git repository:

```bash
./new-semver
```

Prints a JSON metadata object to stdout and updates the `VERSION` file in place.

## How Version Resolution Works

```
Is there a VERSION file?
  ├── Yes → read version from file
  └── No  → fall back to next-version: in GitVersion.yml

Is there at least one commit?
  ├── Yes → parse last commit message, bump version, write VERSION
  └── No  → use GitVersion.yml next-version as-is (no bump)
```

### Conventional Commits → Version Bump

| Commit Prefix | Example | Effect |
|---|---|---|
| `fix:` | `fix: null pointer in auth` | PATCH bump |
| `feat:` | `feat: add retry logic` | MINOR bump |
| `feat!:` or `fix!:` | `feat!: drop v1 API` | MAJOR bump |
| `BREAKING CHANGE:` | `BREAKING CHANGE: schema renamed` | MAJOR bump |
| Anything else (`chore:`, `docs:`, etc.) | `chore: update deps` | No bump |

> Matching is case-insensitive.

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

## Files

| File | Description |
|---|---|
| `VERSION` | Active version in `X.Y.Z` format — managed by the tool, commit this file |
| `GitVersion.yml` | Provides the `next-version:` fallback when `VERSION` doesn't exist |

### GitVersion.yml

```yaml
next-version: 0.0.1       # used when VERSION file is absent
mode: ContinuousDeployment
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

> The container has `git` installed. Mount your repo root to `/workspace` so the tool can read `.git/`, `VERSION`, and `GitVersion.yml`.

---

## CI/CD Integration

### Quick Start (any CI)

Add a `VERSION` file to your repo root so versions persist between pipeline runs:

```bash
echo "0.1.0" > VERSION
git add VERSION
git commit -m "chore: add initial VERSION file"
```

Then run go-semver at the start of your pipeline to bump the version and capture the output:

```bash
VERSION_JSON=$(docker run --rm -v $(pwd):/workspace -w /workspace \
  ghcr.io/miraccan00/go-semver:latest)

# Extract the version string
APP_VERSION=$(echo "$VERSION_JSON" | jq -r '.MajorMinorPatch')
echo "Building version: $APP_VERSION"
```

---

### GitHub Actions

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0          # required — go-semver reads git log

      - name: Run go-semver
        id: semver
        run: |
          VERSION_JSON=$(docker run --rm -v $(pwd):/workspace -w /workspace \
            ghcr.io/miraccan00/go-semver:latest)
          echo "json=$VERSION_JSON" >> $GITHUB_OUTPUT
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
  DOCKER_DRIVER: overlay2

semver:
  image: ghcr.io/miraccan00/go-semver:latest
  stage: .pre
  script:
    - VERSION_JSON=$(new-semver)
    - echo "APP_VERSION=$(echo $VERSION_JSON | jq -r '.MajorMinorPatch')" >> build.env
    - echo "SHORT_SHA=$(echo $VERSION_JSON | jq -r '.ShortSha')" >> build.env
  artifacts:
    reports:
      dotenv: build.env   # makes APP_VERSION available in downstream jobs

build:
  stage: build
  needs: [semver]
  script:
    - docker build -t myapp:$APP_VERSION .
```

> **Important:** Set `GIT_DEPTH: 0` in your GitLab project's CI/CD variables or add it to the job to ensure full git history is available.

```yaml
variables:
  GIT_DEPTH: 0
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

These scenarios are **not fully handled** — keep them in mind when integrating into CI/CD:

| Scenario | Current Behavior |
|---|---|
| Detached HEAD (typical CI checkout) | `BranchName` is empty string, no error raised |
| Pre-release versions (`1.0.0-rc.1`) | Fields exist in `VersionInfo` but are never populated |
| Malformed `VERSION` content (e.g. `abc`) | `strconv.Atoi` silently returns `0` |
| Merge commit messages | No conventional prefix → no bump |
| Multiple bump rules in one commit | First match wins: major → minor → patch |
| `git` binary not found | Git fields return zero values, no error |
| Empty repository (no commits yet) | GitVersion.yml fallback runs; git fields are empty strings |

## Testing

```bash
go test ./internal/semver/...
```

Current coverage: file read/write round-trip, `ParseVersion`, `BumpByCommitMessage` (6 scenarios).

## Project Structure

```
cmd/new-semver/main.go          # CLI entry point
internal/semver/semver.go       # Core library
internal/semver/semver_test.go  # Unit tests
GitVersion.yml                  # Fallback configuration
VERSION                         # Active version file (do not gitignore)
```

## Contributing

Pull requests are welcome. When adding features, please cover edge cases with tests in `internal/semver/semver_test.go`.
