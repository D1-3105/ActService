package gitCmd

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
	Path    string
}
