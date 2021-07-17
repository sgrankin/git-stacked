//git-review will create PRs for review

package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object/commitgraph"

	"github.com/sgrankin/git-stacked/internal/change"
	"github.com/sgrankin/git-stacked/internal/git"
	gh2 "github.com/sgrankin/git-stacked/internal/github"
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func main() {
	flag.Parse()
	ctx := context.Background()

	repo, err := git.Open()
	if err != nil {
		log.Fatalf("git open: %v", err)
	}
	remoteName, remoteURL, err := repo.GetCurrentRemoteURL()
	if err != nil {
		log.Fatalf("getDefaultBranch: %s", err)
	}

	gh, err := gh2.Discover(ctx, remoteURL)
	if err != nil {
		log.Fatalf("discover: %s", err)
	}
	base := gh.DefaultBranch()

	// On the assumption the remote branch is always at least as up to date as the
	// local, we will diff the current head against the remote.  This makes sure we get the right commits:
	// - when one does a remote fetch but does not pull into the local main
	// - when one is stacking commits _on_ the local main (without pushing it)
	baseHash, err := repo.ResolveRevision(remoteName + "/" + base)
	if err != nil {
		log.Fatal(err)
	}
	baseCommits, err := getAllCommits(repo, *baseHash)
	if err != nil {
		log.Fatal(err)
	}

	headHash, err := repo.ResolveRevision(string(plumbing.HEAD))
	if err != nil {
		log.Fatal(err)
	}

	commits, err := getNewCommits(repo, *headHash, baseCommits)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(commits)

	changes, err := ensureChangeID(gh, repo, commits)
	if err != nil {
		log.Fatal(err)
	}

	if err := doPush(gh, repo, changes); err != nil {
		log.Fatal(err)
	}
	if err := syncPRs(gh, repo, changes); err != nil {
		log.Fatal(err)
	}
}

func getAllCommits(repo *git.Repo, start plumbing.Hash) (map[plumbing.Hash]bool, error) {
	seen := map[plumbing.Hash]bool{}
	err := repo.WalkCommits(start, func(cn commitgraph.CommitNode) error {
		seen[cn.ID()] = true
		return nil
	})
	return seen, err
}

func getNewCommits(repo *git.Repo, start plumbing.Hash, end map[plumbing.Hash]bool) ([]plumbing.Hash, error) {
	var result []plumbing.Hash
	if err := repo.WalkCommits(start, func(cn commitgraph.CommitNode) error {
		if end[cn.ID()] {
			return git.SkipCommit
		}
		// TODO: error on merge commits?
		result = append(result, cn.ID())
		return nil
	}); err != nil {
		return nil, err
	}

	// Reverse the results so that the commits are in commit-time order.
	for i := len(result)/2 - 1; i >= 0; i-- {
		opp := len(result) - 1 - i
		result[i], result[opp] = result[opp], result[i]
	}
	return result, nil
}

type Change struct {
	*change.Change
	Head string
	Base string
}

func ensureChangeID(gh *gh2.Client, repo *git.Repo, commits []plumbing.Hash) ([]*Change, error) {
	if len(commits) == 0 {
		return nil, nil
	}
	lastHead := gh.DefaultBranch()

	var changes []*Change
	var lastHash *plumbing.Hash
	for _, h := range commits {
		change, err := change.Ensure(repo, h, lastHash)
		if err != nil {
			return nil, err
		}
		lastHash = &change.Hash
		c := &Change{
			Change: change,
			Head:   fmt.Sprintf("%s/%s", gh.Username(), change.ID),
			Base:   lastHead,
		}
		changes = append(changes, c)
		lastHead = c.Head

	}
	return changes, repo.UpdateHead(*lastHash)
}

func syncPRs(gh *gh2.Client, repo *git.Repo, changes []*Change) error {
	ctx := context.Background()
	for _, c := range changes {
		pr, err := gh.GetPull(ctx, c.Head)
		if err != nil {
			return err
		}
		if pr == nil {
			pr, err = gh.CreatePull(ctx, c.Head, c.Base, c.Title, c.Body)
		} else {
			pr, err = gh.UpdatePull(ctx, *pr.Number, c.Base, c.Title, c.Body)
		}
		if err != nil {
			return err
		}
		log.Printf("Updated PR: %s", *pr.HTMLURL)
	}
	return nil
}

func doPush(gh *gh2.Client, repo *git.Repo, changes []*Change) error {
	refSpecs := map[plumbing.Hash]string{}
	for _, c := range changes {
		refSpecs[c.Hash] = c.Head
	}
	return gh.Push(repo.Storer(), refSpecs)
}
