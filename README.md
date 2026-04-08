# gh-commandeer

Add commits to a contributor's pull request and push them back — without leaving the terminal.

When a contributor opens a PR from their fork, you can check out their branch, make changes, and push directly to their PR. The manual process involves four git commands and knowing where to find the fork URL. `gh-commandeer` handles all of that from a PR number alone.

## Why commandeer a PR?

**Fix minor issues without round-trips** — The contributor's code is 95% right but has a typo, lint failure, or CI breakage. Rather than commenting and waiting for them to fix it, the maintainer just fixes it directly and merges.

**Rebase onto current main** — A PR goes stale because main has moved. The maintainer rebases the contributor's branch to resolve conflicts, since the contributor may be unavailable or unfamiliar with the rebase workflow.

**Apply project style/conventions** — The contribution works but doesn't follow local patterns. The maintainer normalizes it rather than blocking the PR with style comments.

**Unblock a stalled PR** — The contributor opened a good PR but went quiet (job change, vacation, lost interest). Rather than closing and reimplementing from scratch, the maintainer builds on the existing work.

**Add tests or documentation** — Maintainers often have standards the contributor didn't meet (e.g., "all PRs need tests"). Instead of rejection, the maintainer adds the missing pieces and ships it.

**Resolve merge conflicts collaboratively** — In large repos, a contributor may not have the context to resolve conflicts correctly (e.g., they don't know which side of a conflicting migration is correct). The maintainer handles it.

> **Note:** The contributor must have [Allow edits from maintainers](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/working-with-forks/allowing-changes-to-a-pull-request-branch-created-from-a-fork) enabled on the PR — GitHub enforces that you can only push to someone else's fork branch with their explicit permission.

## Installation

```sh
go install github.com/StevenACoffman/gh-commandeer@latest
```

## Setup

Export a [GitHub personal access token](https://github.com/settings/tokens) with `repo` scope:

```sh
export GITHUB_TOKEN=ghp_...
```

Run commands from inside the repository the PR targets (the one with `origin` pointing at GitHub).

## Usage

```sh
gh-commandeer <pr-number>             # check out the contributor's branch
# ... make your commits ...
gh-commandeer push <pr-number>        # push back to their PR
```

## Walkthrough

`alice` has opened PR #42 against your repository. Her changes are on the `feature` branch of her fork. You want to make a few commits and push them to her PR.

### 1. Check out the contributor's branch

```sh
gh-commandeer 42
```

This adds Alice's fork as a remote, fetches it, checks out her branch, and sets its upstream tracking ref:

```
# equivalent to:
git remote add alice https://github.com/alice/repo.git
git fetch alice
git checkout -b alice/feature alice/feature
git branch --set-upstream-to=alice/feature alice/feature
```

You are now on a local branch `alice/feature` with `alice` set as its upstream. Make whatever commits you need.

Because the upstream is configured, the standard git commands work without extra arguments:

```sh
git status   # shows how far ahead/behind you are relative to alice/feature
git push     # pushes to alice's fork branch directly
git pull     # pulls from alice's fork branch directly
```

> **Note:** If Alice's PR does not have [Allow edits from maintainers](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/working-with-forks/allowing-changes-to-a-pull-request-branch-created-from-a-fork) enabled, `gh-commandeer` will warn you. Alice needs to enable this on the PR before you can push.

### 2. Rebase (optional)

To place your commits on top of the current base branch before pushing:

```sh
git rebase origin/main
```

### 3. Push back to the PR

```sh
gh-commandeer push 42
```

If you rebased, add `--force`:

```sh
gh-commandeer push --force 42
```

```
# equivalent to:
git push alice alice/feature:feature
git push --force alice alice/feature:feature  # after rebase or amend
```

Alice's PR now contains your commits.

## Command reference

### Root command

```
gh-commandeer [FLAGS] <pr-number>
```

Looks up the PR via the GitHub API, adds the contributor's fork as a remote (named after their login), fetches it, and checks out their branch as `<login>/<branch>`.

| Flag | Default | Description |
|------|---------|-------------|
| `--token` | `$GITHUB_TOKEN` | GitHub personal access token |
| `--owner` | from `origin` remote | Repository owner |
| `--repo` | from `origin` remote | Repository name |

### `push`

```
gh-commandeer push [FLAGS] <pr-number>
```

Looks up the PR to find the remote name and target branch, then pushes the current local branch to the contributor's fork.

| Flag | Default | Description |
|------|---------|-------------|
| `--token` | `$GITHUB_TOKEN` | GitHub personal access token |
| `--owner` | from `origin` remote | Repository owner |
| `--repo` | from `origin` remote | Repository name |
| `--force` | false | Force-push (use after `git rebase` or `git commit --amend`) |

`--owner` and `--repo` are auto-detected from your `origin` remote and rarely need to be set explicitly.
