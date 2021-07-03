//git-review will create PRs for review
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object/commitgraph"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

var (
	onto = flag.String("onto", "", "which branch, ref or commit we should target with reviews; defaults to master")
)

func main() {
	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Getwd: %v", err)
	}
	base, err := determineBaseBranch(*onto)
	if err != nil {
		log.Fatal(err)
	}

	repo, err := git.PlainOpenWithOptions(wd, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		log.Fatalf("git open: %v", err)
	}

	baseHash, err := repo.ResolveRevision(plumbing.Revision(base))
	if err != nil {
		log.Fatal(err)
	}
	headHash, err := repo.ResolveRevision(plumbing.Revision(plumbing.HEAD))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(baseHash)
	fmt.Println(headHash)

	cni := commitgraph.NewObjectCommitNodeIndex(repo.Storer)
	baseCommits, err := getAllCommits(cni, *baseHash)
	if err != nil {
		log.Fatal(err)
	}

	commits, err := getNewCommits(cni, *headHash, baseCommits)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(commits)

	// Ensure all commits have change ID.  update current ref if necessary
	// Force-push new branches
	// Create PRs

	if err := ensureChangeID(base); err != nil {
		log.Fatal(err)
	}
	if err := doPush(); err != nil {
		log.Fatal(err)
	}
}

func getAllCommits(cni commitgraph.CommitNodeIndex, start plumbing.Hash) (map[plumbing.Hash]bool, error) {
	pending := []plumbing.Hash{start}
	seen := map[plumbing.Hash]bool{}
	for len(pending) > 0 {
		var h plumbing.Hash
		pending, h = pending[:len(pending)-1], pending[len(pending)-1]
		n, err := cni.Get(h)
		if err != nil {
			return nil, err
		}
		if seen[n.ID()] {
			continue
		}
		seen[n.ID()] = true
		pending = append(pending, n.ParentHashes()...)
	}
	return seen, nil
}

func getNewCommits(cni commitgraph.CommitNodeIndex, start plumbing.Hash, end map[plumbing.Hash]bool) ([]plumbing.Hash, error) {
	var result []plumbing.Hash
	hash := start
	for {
		if end[hash] {
			return result, nil
		}
		result = append(result, hash)
		cn, err := cni.Get(start)
		if err != nil {
			return nil, err
		}
		parents := cn.ParentHashes()
		if len(parents) > 1 {
			log.Printf("found merge commit; ending search for commits not in base: %s", hash)
			return result, nil
		} else if len(parents) == 0 {
			return result, nil
		}
		hash = parents[0]
	}
}

// resolveName translates the string (which may be a hash, branch, or ref name) into the hash.
func resolveName(s storer.Storer, name string) (plumbing.Hash, error) {
	if ref, err := storer.ResolveReference(s, plumbing.NewBranchReferenceName(name)); err == nil {
		return ref.Hash(), nil
	}
	if ref, err := storer.ResolveReference(s, plumbing.NewTagReferenceName(name)); err == nil {
		return ref.Hash(), nil
	}
	if ref, err := storer.ResolveReference(s, plumbing.ReferenceName(name)); err == nil {
		return ref.Hash(), nil
	}
	h := plumbing.NewHash(name)
	return h, s.HasEncodedObject(h)
}

func determineBaseBranch(onto string) (string, error) {
	if onto != "" {
		return onto, nil
	}
	// TODO: check config, check defaults, check github default branch
	return "master", nil
}

// TODO: should get commits as arg instead of base
func ensureChangeID(base string) error {
	return nil

}
func doPush() error {
	return nil
}

// * Know what the base branch is:
//   * Default: `origin/master`
//   * Default: the remote's (github) default branch
//   * Maybe: Configure the branch with `git review --onto <branch>`
// * Find commits that are not present in the branch
// * Update all of the commit messages that do not have a `Change-ID:` header.
//   * Rebase in place, effectively, only modifying the commit message.
// * For each change:
//   * Force-push a ref named with the change-id.
//   * Create or update the PR onto the base branch (if first commit) or onto previous change.
