//git-review will create PRs for review
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object/commitgraph"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v36/github"
	"github.com/segmentio/ksuid"
	"golang.org/x/oauth2"
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
	// Force-push new branches
	// Create PRs

	commits, err = ensureChangeID(repo, commits)
	if err != nil {
		log.Fatal(err)
	}
	if err := doPush(repo, commits); err != nil {
		log.Fatal(err)
	}
	if err := syncPRs(repo, commits); err != nil {
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
		log.Printf("%s", hash)
		if end[hash] {
			break
		}
		result = append(result, hash)
		cn, err := cni.Get(hash)
		if err != nil {
			return nil, err
		}
		parents := cn.ParentHashes()
		if len(parents) > 1 {
			log.Printf("found merge commit; ending search for commits not in base: %s", hash)
			break
		} else if len(parents) == 0 {
			break
		}
		hash = parents[0]
	}
	log.Printf("done %s", hash)

	// Reverse the results so that the commits are in commit-time order.
	for i := len(result)/2 - 1; i >= 0; i-- {
		opp := len(result) - 1 - i
		result[i], result[opp] = result[opp], result[i]
	}
	return result, nil
}

func determineBaseBranch(onto string) (string, error) {
	if onto != "" {
		return onto, nil
	}
	// TODO: check config, check defaults, check github default branch
	return "master", nil
}

// TODO: should get commits as arg instead of base
func ensureChangeID(repo *git.Repository, commits []plumbing.Hash) ([]plumbing.Hash, error) {
	var prevOldHash plumbing.Hash
	var prevNewHash plumbing.Hash
	var newCommits []plumbing.Hash

	// - ensure change ID is allocated (and save it)
	// - name the branch; it will only exist remotely though!
	// - ensure parent branch has been noted?

	for _, h := range commits {
		c, err := repo.CommitObject(h)
		if err != nil {
			return nil, err
		}
		if !prevOldHash.IsZero() && !cmp.Equal([]plumbing.Hash{prevOldHash}, c.ParentHashes) {
			return nil, fmt.Errorf("for commit %s, got parent %v but wanted %v", c.Hash, c.ParentHashes, prevOldHash)
		}
		prevOldHash = c.Hash

		message := appendChangeID(c.Message, ksuid.New().String())
		if message != c.Message {
			log.Printf("will update message for commit %v: %q", c.Hash, message)
		}
		c.Message = message
		obj := repo.Storer.NewEncodedObject()
		if err := c.Encode(obj); err != nil {
			return nil, err
		}
		prevNewHash, err = repo.Storer.SetEncodedObject(obj)
		if err != nil {
			return nil, err
		}
		newCommits = append(newCommits, prevNewHash)
	}
	head, err := repo.Storer.Reference(plumbing.HEAD)
	if err != nil {
		return nil, err
	}

	name := plumbing.HEAD
	if head.Type() != plumbing.HashReference {
		name = head.Target()
	}

	ref := plumbing.NewHashReference(name, prevNewHash)
	return newCommits, repo.Storer.SetReference(ref)
}

func getChangeID(repo *git.Repository, commit plumbing.Hash) (string, error) {
	c, err := repo.CommitObject(commit)
	if err != nil {
		return "", err
	}
	matches := extractChangeID.FindStringSubmatch(c.Message)
	if len(matches) != 2 {
		return "", fmt.Errorf("%s trailer with vaule not found in %q", changeIDToken, c.Message)
	}
	return matches[1], nil
}

func syncPRs(repo *git.Repository, commits []plumbing.Hash) error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")})
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	branches := []string{"master"}
	for _, c := range commits {
		changeID, err := getChangeID(repo, c)
		if err != nil {
			return err
		}
		branches = append(branches, fmt.Sprintf("%s/%s", "sgrankin", changeID))
	}

	for i := 1; i < len(branches); i++ {
		// base := branches[i-1]
		head := branches[i]
		prs, _, err := client.PullRequests.List(ctx, "sgrankin", "git-stakced", &github.PullRequestListOptions{Head: head})
		if err != nil {
			return err
		}
		fmt.Println(prs)
	}

	return nil
}

func doPush(repo *git.Repository, commits []plumbing.Hash) error {
	var refSpecs []config.RefSpec
	for _, commit := range commits {
		changeID, err := getChangeID(repo, commit)
		if err != nil {
			return err
		}
		// TODO: extract username from github config
		// TODO: common branch format .. somewhere
		refSpecs = append(refSpecs, config.RefSpec(fmt.Sprintf("%s:refs/heads/%s/%s", commit.String(), "sgrankin", changeID)))
	}

	log.Printf("token: %q", os.Getenv("GITHUB_TOKEN"))
	log.Printf("refspecs: %v", refSpecs)
	return git.
		NewRemote(repo.Storer, &config.RemoteConfig{
			Name: "github",
			// TODO: extract repo info from the configured remote
			URLs: []string{"https://github.com/sgrankin/git-stacked.git"},
		}).
		Push(&git.PushOptions{
			RemoteName: "github",
			RefSpecs:   refSpecs,
			// Auth:       &http.TokenAuth{Token: os.Getenv("GITHUB_TOKEN")},
			Auth:     &http.BasicAuth{Username: "sgrankin", Password: os.Getenv("GITHUB_TOKEN")},
			Progress: os.Stderr,
			Force:    true,
		})
}

const changeIDToken = "Change-ID"

var hasChangeID = regexp.MustCompile(`(?m)^` + changeIDToken + `\s*:\s*`)
var hasTrailers = regexp.MustCompile(`(?s)^(.+\n\n|\n*)(\S[^:\n]*:\s*[^\n]*(\n\s+\S*)*)(\n\S+\s*:\s*\S*(\n\s+\S*)*)*\n?$`)
var extractChangeID = regexp.MustCompile(`(?m)^` + changeIDToken + `\s*:\s*(.*)$`)

func appendChangeID(message, changeID string) string {
	if hasChangeID.MatchString(message) {
		return message
	}
	trailer := fmt.Sprintf("%s: %s\n", changeIDToken, changeID)
	if message == "" {
		return trailer
	}
	message = strings.TrimSuffix(message, "\n")
	if !hasTrailers.MatchString(message) {
		message += "\n"
	}
	message += "\n" + trailer
	return message
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
