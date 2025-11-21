package gitCmd

import (
	"errors"
	"fmt"
	"github.com/D1-3105/ActService/conf"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	var cmd *exec.Cmd

	if err := os.MkdirAll(pth, 0755); err != nil {
		glog.Errorf("Error creating directory %s: %v", pth, err)
		return nil, err
	}

	if gitEnviron.GithubRequireToken && strings.HasPrefix(gf.Repo.Url, "http") {
		authUrl := strings.Replace(
			gf.Repo.Url, "https://", fmt.Sprintf("https://x-token:%s@", gitEnviron.GithubToken), 1,
		)
		authUrl = strings.Replace(
			authUrl, "http://", fmt.Sprintf("http://x-token:%s@", gitEnviron.GithubToken), 1,
		)

		cmd = exec.Command("git", "clone", authUrl, pth)

	} else if gitEnviron.GithubRequireSsh && strings.HasPrefix(gf.Repo.Url, "git@") {
		cmd = exec.Command("git", "clone", gf.Repo.Url, pth)

		sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=yes", gitEnviron.GithubPrivateSsh)
		cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_SSH_COMMAND=%s", sshCmd))

	} else {
		cmd = exec.Command("git", "clone", gf.Repo.Url, pth)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		glog.Errorf("Error cloning git repo %s: %v\nOutput: %s", gf.Repo.Url, err, string(output))
		_ = os.RemoveAll(pth)
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(pth, ".git")); os.IsNotExist(err) {
		glog.Errorf("Git repository was not cloned properly to %s", pth)
		_ = os.RemoveAll(pth)
		return nil, errors.New("git repo clone failed")
	}
	checkoutCmd := exec.Command("git", "checkout", gf.Repo.CommitId)
	checkoutCmd.Dir = pth

	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		glog.Errorf(
			"Error checking out git repo %s@%s: %v\nOutput: %s",
			gf.Repo.Url, gf.Repo.CommitId, err, string(output),
		)
		_ = os.RemoveAll(pth)
		return nil, err
	}
	return &ClonedRepo{
		gf.Repo, pth,
	}, nil
}

func (clone ClonedRepo) Dispose() error {
	teardown, err := strconv.ParseBool(os.Getenv("ACT_TEARDOWN_GIT_FOLDER"))
	if err != nil {
		teardown = true
	}
	gitEnviron := conf.GitEnv{
		TeardownFolder: teardown,
	}
	glog.V(2).Infof("gitEnviron.TeardownFolder = %t", gitEnviron.TeardownFolder)
	if gitEnviron.TeardownFolder {
		glog.V(1).Infof("Removing cloned git repo %s!", clone.Path)
		err := os.RemoveAll(clone.Path)
		return err
	}
	return nil
}
