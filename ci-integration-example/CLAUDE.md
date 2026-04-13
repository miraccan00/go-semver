# CLAUDE.md — Versioning

This repository uses **[go-semver](https://github.com/miraccan00/go-semver)** for automated semantic versioning.
Version numbers are computed entirely from **Conventional Commit** messages and **git tags** — no VERSION file is written or committed.

## How versions are determined

```
Is there a semver git tag (e.g. v1.2.3)?
  ├── No  → read next-version: from GitVersion.yml → output as-is
  └── Yes → collect commits since that tag
              └── Any new commits?
                    ├── No  → output tag version unchanged
                    └── Yes → on mainline + merge detected?
                                ├── Yes → bump by commits + create git tag
                                └── No  → bump by commits (no tag)
```

The bump level is **always commit-driven**. Branch names never affect bump magnitude.

---

## Branch Strategy

```
Branch        Merges into          Bump              Auto-tag
────────────────────────────────────────────────────────────
main          —                    —                 yes (on merge)
develop       main                 commit-driven     on merge to main
release/*     main + develop       commit-driven     on merge to main
hotfix/*      main + develop       commit-driven     on merge to main
feature/*     develop              none              no
```

- `feature/*` branches from `develop`, deleted after merge.
- `release/*` branches from `develop`, merges into **both** `main` and `develop`.
- `hotfix/*` branches from `main`, merges into **both** `main` and `develop`.

---

## Conventional Commits — Required Format

Every commit message **must** follow this format:

```
<type>[optional scope]: <short description>

[optional body]

[optional footer(s)]
```

### Bump reference

| Prefix | Example | Version bump |
|---|---|---|
| `fix:` / `fix(scope):` | `fix: null pointer in login` | PATCH |
| `feat:` / `feat(scope):` | `feat(api): add pagination` | MINOR |
| `feat!:` / `fix!:` / `feat(scope)!:` | `feat!: remove legacy endpoint` | MAJOR |
| `BREAKING CHANGE:` in footer | see below | MAJOR |
| `BREAKING-CHANGE:` in footer | see below | MAJOR |
| `chore:`, `docs:`, `test:`, `ci:`, etc. | `chore: update deps` | no bump |

Breaking change via footer (alternative to `!`):

```
feat: migrate auth to OAuth2

BREAKING CHANGE: previous API key auth is no longer supported
```

- Matching is **case-insensitive**.
- Scopes are optional and do not affect bump level.
- When multiple commits are present the **highest** bump wins.
- Only `fix:` and `feat:` (and their `!` variants) affect the version.

---

## How to Write Commits

1. **Always use a Conventional Commit prefix.** Never write bare messages like `"update config"` — use `chore: update config`.
2. **Choose the prefix that matches the change:**
   - Bug fix → `fix:`
   - New capability → `feat:`
   - Breaking API/behaviour change → `feat!:` or add `BREAKING CHANGE:` footer
   - Build/tooling/deps → `chore:`
   - Documentation → `docs:`
   - Tests → `test:`
   - CI pipeline → `ci:`
   - Code cleanup with no behaviour change → `refactor:`
3. **Keep the subject line under 72 characters.**
4. **Do not use past tense.** Write `fix: handle nil pointer` not `fixed null pointer`.
5. **Add scope when the change is scoped to a module or subsystem**: `feat(auth): add OAuth2 support`.

### Examples

```
feat(api): add rate limiting middleware
fix: correct off-by-one error in pagination
chore: upgrade Go to 1.24
docs: add setup instructions to README
feat!: replace REST API with gRPC

BREAKING CHANGE: all existing REST clients must be migrated to gRPC
```

---

## Key Rules

- **Never suggest bumping the version manually** — the version is always computed from commits.
- **Never create or edit a VERSION file** — versioning is tag-based.
- **Always use Conventional Commit prefixes** in any commit message you write or suggest.
- **Remind the user to set `SOURCE_BRANCH`** when reviewing CI configs for squash-merge workflows.
- **Warn if `fetch-depth: 0` is missing** from CI checkout steps — shallow clones break go-semver.
- When a PR includes only `chore:`, `docs:`, or `test:` commits, inform the user that **no version bump** will occur on merge.
- When a PR includes a `feat!:` commit or a `BREAKING CHANGE:` footer, remind the user that a **MAJOR bump** will fire on merge to mainline.
