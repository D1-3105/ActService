package tests

import (
	"encoding/json"
	"github.com/D1-3105/ActService/conf"
	"github.com/D1-3105/ActService/pkg/actCmd"
	"github.com/D1-3105/ActService/pkg/gitCmd"
	"github.com/golang/glog"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixtureTestConf(t *testing.T) map[string]string {
	fixturesDir, err := filepath.Abs("./fixtures/")
	require.NoError(t, err)

	if os.Getenv("LOCAL") == "1" {
		fixturesDir = filepath.Join(fixturesDir, "local")
	}

	testFilePath := filepath.Join(fixturesDir, "act_call_test.json")
	testFile, err := os.OpenFile(testFilePath, os.O_RDONLY, os.ModePerm)
	require.NoError(t, err)
	defer func(testFile *os.File) {
		_ = testFile.Close()
	}(testFile)

	var fileContent map[string]map[string]string
	err = json.NewDecoder(testFile).Decode(&fileContent)
	require.NoError(t, err)

	cnf, ok := fileContent[t.Name()]
	require.Truef(t, ok, "no config found for test name %s in %s", t.Name(), testFilePath)
	return cnf
}

func gitFixture(t *testing.T) (*gitCmd.GitFolder, *gitCmd.ClonedRepo) {
	testConf := fixtureTestConf(t)
	t.Setenv("ACT_BINARY_PATH", testConf["ACT_BINARY_PATH"])
	t.Setenv("ACT_DOCKER_CONTEXT_PATH", testConf["ACT_DOCKER_CONTEXT_PATH"])

	gitFolder, err := gitCmd.NewGitFolder(
		&gitCmd.GitRepo{
			Url:      "https://github.com/cplee/github-actions-demo.git",
			CommitId: "5c6f585b1f9d8526c8e1672c5f8f00883b895d93",
		},
		"/dev/shm/tests",
	)
	require.NoError(t, err)
	clone, err := gitFolder.Clone()
	require.NoError(t, err)
	return gitFolder, clone
}

func actCmdFixture(t *testing.T) (*actCmd.ActCommand, *gitCmd.ClonedRepo) {
	_, clone := gitFixture(t)
	var actEnviron conf.ActEnviron
	conf.NewEnviron(&actEnviron)
	actCommand := actCmd.NewActCommand(
		&actEnviron, "-P ubuntu-latest=node:16-buster", clone.Path,
	)
	return actCommand, clone
}

func TestActCall(t *testing.T) {
	actCommand, cloned := actCmdFixture(t)
	defer func() { _ = cloned.Dispose() }()
	output, err := actCommand.Call(t.Context())
	require.NoError(t, err)
	defer output.Close()

	timeout := time.After(1000 * time.Second)
	for {
		select {
		case out := <-output.GetOutputChan():
			text := out.FormatRead()
			glog.Info(text)
			glog.Flush()
			break
		case exitCode := <-output.GetExitCode():
			glog.Infof("Exit code: %d", exitCode)
			require.Equal(t, 0, exitCode)
			return
		case err := <-output.ProgramError():
			if err != nil {
				t.Fatalf("Process error: >>%v<<;", err)
			}
		case <-timeout:
			t.Fatalf("Timeout! Struct: %v", output)
		}
	}
}
