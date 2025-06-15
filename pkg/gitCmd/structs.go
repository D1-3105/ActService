package gitCmd

import "github.com/go-git/go-git/v5"

type GitRepo struct {
	Url      string
	CommitId string
}

type GitFolder struct {
	Repo *GitRepo
	Path string
}

type ClonedRepo struct {
	gitRepo *GitRepo
	cloned  *git.Repository
	Path    string
}
