# Install
* `sgrankin.dev/git-stacked/cmd/git-review@main`

# Workflow
* Be working in a branch of `master` /` main`
* Create and tend a series of commits.
* When ready for review, run `git review`.  PRs are created, one for each commit, dependent on each other.
* Amend commits as ready.  Re-run `git review` to update all PRs.
* Merge the first commit.
* Pull & rebase your branch.
* `git review` to update PRs.

# Algorithms
### `git-review`
* Know what the base branch is:
  * Default: the remote's (github) default branch
* Find commits that are not present in the branch.
* Update all of the commit messages that do not have a `Change-ID:` header.
  * Rebase in place, effectively, only modifying the commit message.
* For each change:
  * Force-push a ref named with the change-id.
  * Create or update the PR onto the base branch (if first commit) or onto previous change.

# Other tools
* `git-absorb` (https://github.com/tummychow/git-absorb) to make amending a series of commits easy.
