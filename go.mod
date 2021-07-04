module github.com/sgrankin/git-stacked

go 1.16

replace github.com/go-git/go-git/v5 => ../go-git/

require (
	github.com/go-git/go-git/v5 v5.4.2
	github.com/google/go-cmp v0.5.6
	github.com/google/go-github/v36 v36.0.0
	github.com/segmentio/ksuid v1.0.3
	golang.org/x/oauth2 v0.0.0-20180821212333-d2e6202438be
)
