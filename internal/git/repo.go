package git

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/object/commitgraph"
	"github.com/go-git/go-git/v5/storage"
)

type Repo struct {
	repo *git.Repository
}

func Open() (*Repo, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	repo, err := git.PlainOpenWithOptions(wd, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, err
	}
	return &Repo{repo: repo}, nil
}

func (r *Repo) Storer() storage.Storer {
	return r.repo.Storer
}

// updateHead will find the symbolic ref pointed to by HEAD, if any, and set it
// to h.  Effectively, this updates the currently checked out branch, if any.  If
// detached (head is a hash), we update head directly.
func (r *Repo) UpdateHead(h plumbing.Hash) error {
	head, err := r.repo.Storer.Reference(plumbing.HEAD)
	if err != nil {
		return err
	}

	name := plumbing.HEAD
	if head.Type() == plumbing.SymbolicReference {
		name = head.Target()
	}

	return r.repo.Storer.SetReference(plumbing.NewHashReference(name, h))
}

func (r *Repo) ResolveRevision(rev string) (*plumbing.Hash, error) {
	return r.repo.ResolveRevision(plumbing.Revision(rev))
}

func (r *Repo) GetCommit(h plumbing.Hash) (*object.Commit, error) {
	return r.repo.CommitObject(h)
}

func (r *Repo) SaveCommit(c *object.Commit) (plumbing.Hash, error) {
	obj := r.repo.Storer.NewEncodedObject()
	if err := c.Encode(obj); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("commit.Encode: %w", err)
	}
	return r.repo.Storer.SetEncodedObject(obj)
}

type CommitWalkFunc func(commitgraph.CommitNode) error

var SkipCommit = errors.New("skip this commit") //lint:ignore ST1012 Not an error.

func (r *Repo) WalkCommits(h plumbing.Hash, fn CommitWalkFunc) error {
	cni := commitgraph.NewObjectCommitNodeIndex(r.repo.Storer)
	pending := []plumbing.Hash{h}
	seen := map[plumbing.Hash]struct{}{}
	for len(pending) > 0 {
		var h plumbing.Hash
		pending, h = pending[:len(pending)-1], pending[len(pending)-1]
		n, err := cni.Get(h)
		if err != nil {
			return err
		}
		if _, found := seen[n.ID()]; found {
			continue
		}
		seen[n.ID()] = struct{}{}
		err = fn(n)
		if err == SkipCommit {
			continue
		}
		if err != nil {
			return err
		}
		pending = append(pending, n.ParentHashes()...)
	}
	return nil
}

func (r *Repo) GetCurrentRemoteURL() (string, error) {
	// Check if the current branch has a remote configured.
	ref, err := r.repo.Head()
	if err != nil {
		return "", err
	}
	remote := ""
	if ref.Name().IsBranch() {
		br, err := r.repo.Branch(ref.Name().Short())
		if err != nil {
			return "", err
		}
		remote = br.Remote
	}

	remotes, err := r.repo.Remotes()
	if err != nil {
		return "", err
	}

	// Find the first matching remote, return the first URL.
	for _, r := range remotes {
		if remote == "" || r.Config().Name == remote {
			return r.Config().URLs[0], nil
		}
	}
	return "", nil
}
