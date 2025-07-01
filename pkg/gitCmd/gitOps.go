package gitCmd

import (
	"errors"
	"github.com/D1-3105/ActService/conf"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"os"
	"path/filepath"
)

func NewGitFolder(gitRepo *GitRepo, gitNest string) (*GitFolder, error) {
	if gitRepo == nil {
		return nil, errors.New("gitRepo is nil")
	}
	pth := filepath.Join(gitNest)
	err := os.MkdirAll(pth, 0755)
	if err != nil {
		return nil, err
	}
	return &GitFolder{Repo: gitRepo, Path: pth}, nil
}

func (gf *GitFolder) Clone() (*ClonedRepo, error) {

	id := uuid.New()
	pth := filepath.Join(gf.Path, id.String())
	glog.V(1).Infof("Cloning git repo %s -> %s", gf.Repo.Url, pth)
	gitEnviron := conf.GitEnv{}
	conf.NewEnviron(&gitEnviron)
	var err error
	var clone *git.Repository
	if gitEnviron.GithubRequireToken {
		clone, err = git.PlainClone(pth, false, &git.CloneOptions{URL: gf.Repo.Url, Depth: 1, Auth: &http.BasicAuth{
			Username: "x-token",
			Password: gitEnviron.GithubToken,
		}})
	} else {
		clone, err = git.PlainClone(pth, false, &git.CloneOptions{URL: gf.Repo.Url, Depth: 1})
	}
	if err != nil {
		glog.Errorf("Error cloning git repo %s: %v", gf.Repo.Url, err)
		return nil, err
	}
	hash := plumbing.NewHash(gf.Repo.CommitId)
	worktree, err := clone.Worktree()
	if err != nil {
		glog.Errorf("Error cloning git repo %s: %v", gf.Repo.Url, err)
		return nil, err
	}

	if err = worktree.Checkout(&git.CheckoutOptions{Hash: hash}); err != nil {
		glog.Errorf("Error checking out git repo %s: %v", gf.Repo.Url, err)
		return nil, err
	}
	return &ClonedRepo{
		gf.Repo, clone, pth,
	}, nil
}

func (clone ClonedRepo) Dispose() error {
	err := os.RemoveAll(clone.Path)
	return err
}
