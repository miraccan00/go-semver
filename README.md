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
