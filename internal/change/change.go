package change

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/segmentio/ksuid"

	"github.com/sgrankin/git-stacked/internal/git"
)

type Change struct {
	ID    string
	Title string
	Body  string

	Hash    plumbing.Hash
	BaseRef string
	HeadRef string
}

// Ensure opens the commit at hash; if it does not have an associated Change-ID, one
// will be allocated and the commit updated to include it as a trailer.
func Ensure(repo *git.Repo, h plumbing.Hash, base *plumbing.Hash) (*Change, error) {
	commit, err := repo.GetCommit(h)
	if err != nil {
		return nil, err
	}
	c := &Change{}
	c.ID = getChangeID(commit.Message)
	if c.ID == "" {
		c.ID = ksuid.New().String()
		commit.Message = appendChangeID(commit.Message, c.ID)
	}
	c.Title, c.Body = splitMessage(commit.Message)
	if base != nil {
		commit.ParentHashes = []plumbing.Hash{*base}
	}
	c.Hash, err = repo.SaveCommit(commit)
	if err != nil {
		return nil, err
	}
	return c, nil
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

func getChangeID(message string) string {
	matches := extractChangeID.FindStringSubmatch(message)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

func splitMessage(message string) (string, string) {
	splits := strings.SplitN(message, "\n", 2)
	subject := splits[0]
	body := ""
	if len(splits) > 1 {
		body = splits[1]
	}
	return subject, body
}
