# Install
* Clone the repo
* `go install ./cmd/git-review/`

Don't use `go install` or `go get` directly with the repo URL: `replace`
directives in `go.mod` will cause it to fail or not pick up a bug fix, silently
breaking pushes.

# Design

## Interface
* Be working in a branch of `master` or on `master`
* Create and tend commits.
* When ready for review, run `git review`.
* Merge PRs (via rebase) or run `git review --merge` to merge all approved PRs that are ready.
* `git pull --rebase`
* Run `git review` to update all unmerged PRs.

## Algorithms
### `git-review`
* Know what the base branch is:
  * Default: `origin/master`
  * Default: the remote's (github) default branch
  * Maybe: Configure the branch with `git review --onto <branch>`
* Find commits that are not present in the branch
* Update all of the commit messages that do not have a `Change-ID:` header.
  * Rebase in place, effectively, only modifying the commit message.
* For each change:
  * Force-push a ref named with the change-id.
  * Create or update the PR onto the base branch (if first commit) or onto previous change.
