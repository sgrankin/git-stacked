package github

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage"
	"github.com/google/go-github/v36/github"
	"golang.org/x/oauth2"
)

type Client struct {
	token    string
	username string

	owner string
	repo  string

	defaultBranch string
}

var githubURL = regexp.MustCompile(`^(?:https://github.com/|git@github.com:)([^/]+)/(.+).git$`)

func Discover(ctx context.Context, remoteURL string) (*Client, error) {
	log.Println(remoteURL)
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN env must be set to a valid GitHub PAT")
	}
	gh := ghClient(ctx, token)
	user, _, err := gh.Users.Get(ctx, "")
	if err != nil {
		return nil, err
	}

	matches := githubURL.FindStringSubmatch(remoteURL)
	if matches == nil {
		return nil, fmt.Errorf("remote URL %q did not match expected pattern %s", remoteURL, githubURL)
	}
	owner := matches[1]
	repo := matches[2]

	r, _, err := gh.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	defaultBranch := r.DefaultBranch

	return &Client{
		token:         token,
		username:      *user.Login,
		owner:         owner,
		repo:          repo,
		defaultBranch: *defaultBranch,
	}, nil
}

func (c *Client) Username() string {
	return c.username
}
func (c *Client) DefaultBranch() string {
	return c.defaultBranch
}

// `refs` is a map from hash to remote head name.
func (c *Client) Push(storer storage.Storer, refs map[plumbing.Hash]string) error {
	log.Printf("Pushing refs: %v", refs)
	var refSpecs []config.RefSpec
	for h, ref := range refs {
		refSpecs = append(refSpecs, config.RefSpec(
			fmt.Sprintf("%s:refs/heads/%s", h, ref)))
	}

	err := git.
		NewRemote(storer, &config.RemoteConfig{
			Name: "github",
			URLs: []string{fmt.Sprintf("https://github.com/%s/%s.git", c.owner, c.repo)},
		}).
		Push(&git.PushOptions{
			RemoteName: "github",
			RefSpecs:   refSpecs,
			Auth: &http.BasicAuth{
				Username: c.username,
				Password: c.token,
			},
			Progress: os.Stderr,
			Force:    true,
		})
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

func ghClient(ctx context.Context, token string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func (c *Client) GetPull(ctx context.Context, head string) (*github.PullRequest, error) {
	ghClient := ghClient(ctx, c.token)
	prs, _, err := ghClient.PullRequests.List(ctx, c.owner, c.repo, &github.PullRequestListOptions{
		Head: fmt.Sprintf("%s:%s", c.owner, head),
	})
	if err != nil {
		return nil, err
	}
	if len(prs) > 1 {
		log.Fatalf("Found multiple PRs for head: %s", head)
	}
	if len(prs) == 0 {
		return nil, nil
	}
	return prs[0], nil
}

func (c *Client) CreatePull(ctx context.Context, head, base, title, body string) (*github.PullRequest, error) {
	ghClient := ghClient(ctx, c.token)
	pr, _, err := ghClient.PullRequests.Create(ctx,
		c.owner, c.repo, &github.NewPullRequest{
			Head:  &head,
			Base:  &base,
			Title: &title,
			Body:  &body,
		})
	return pr, err
}

func (c *Client) UpdatePull(ctx context.Context, pull int, base, title, body string) (*github.PullRequest, error) {
	ghClient := ghClient(ctx, c.token)
	pr, _, err := ghClient.PullRequests.Edit(ctx,
		c.owner, c.repo, pull, &github.PullRequest{
			Base:  &github.PullRequestBranch{Ref: &base},
			Title: &title,
			Body:  &body,
		})
	return pr, err
}
