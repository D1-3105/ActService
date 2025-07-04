package tests

import (
	"context"
	"encoding/json"
	actservice "github.com/D1-3105/ActService/api/gen/ActService"
	"github.com/D1-3105/ActService/api/rpc"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/D1-3105/ActService/internal/ActService_listen_file"
	"github.com/stretchr/testify/require"
)

func fixtureJobServiceTestConf(t *testing.T) map[string]string {
	fixturesDir, err := filepath.Abs("./fixtures/")
	require.NoError(t, err)

	if os.Getenv("LOCAL") == "1" {
		fixturesDir = filepath.Join(fixturesDir, "local")
	}

	testFilePath := filepath.Join(fixturesDir, "job_service_test.json")
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

func TestScheduleActJob_to_JobLogStream(t *testing.T) {
	testConf := fixtureJobServiceTestConf(t)
	t.Setenv("ACT_BINARY_PATH", testConf["ACT_BINARY_PATH"])
	t.Setenv("ACT_DOCKER_CONTEXT_PATH", testConf["ACT_DOCKER_CONTEXT_PATH"])
	t.Setenv("LOG_FILE_STORAGE", testConf["LOG_FILE_STORAGE"])

	svc := rpc.ActService{
		FileListenersPool: ActService_listen_file.NewFileListeners(),
		JobCtxCancels:     make(map[string]context.CancelFunc),
	}

	resp, err := svc.ScheduleActJob(context.Background(), &actservice.Job{
		RepoUrl:  testConf["repo_url"],
		CommitId: testConf["commit_id"],
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.JobId)

	cancelFunc, exists := svc.JobCtxCancels[resp.JobId]
	require.True(t, exists, "expected jobCtxCancels to contain job id")
	require.NotNil(t, cancelFunc)

	stream := NewMockStream()
	req := &actservice.JobLogRequest{
		JobId:      resp.JobId,
		LastOffset: 0,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.JobLogStream(req, stream)
	}()

	timeout := time.After(30 * time.Second)
	messageCount := 0

messageLoop:
	for {
		select {
		case msg := <-stream.messages:
			t.Logf("[%s] %s", msg.Type.String(), msg.Line)
			require.NotEmpty(t, msg.Line)
			messageCount++
			continue
		case <-timeout:
			cancelFunc()
			require.NoError(t, <-errCh)
			if messageCount < 20 {
				require.Fail(t, "timed out waiting for job log messages")
			}
			break messageLoop
		}
	}

	time.Sleep(1 * time.Second)

	logFilePath := filepath.Join(testConf["LOG_FILE_STORAGE"], resp.JobId)
	_ = os.Remove(logFilePath)
}

func TestScheduleActJob_and_Cancel(t *testing.T) {
	testConf := fixtureJobServiceTestConf(t)
	t.Setenv("ACT_BINARY_PATH", testConf["ACT_BINARY_PATH"])
	t.Setenv("ACT_DOCKER_CONTEXT_PATH", testConf["ACT_DOCKER_CONTEXT_PATH"])
	t.Setenv("LOG_FILE_STORAGE", testConf["LOG_FILE_STORAGE"])

	svc := rpc.ActService{
		FileListenersPool: ActService_listen_file.NewFileListeners(),
		JobCtxCancels:     make(map[string]context.CancelFunc),
	}
	workflowFile := testConf["workflow_file"]
	resp, err := svc.ScheduleActJob(context.Background(), &actservice.Job{
		RepoUrl:      testConf["repo_url"],
		CommitId:     testConf["commit_id"],
		WorkflowFile: &workflowFile,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.JobId)

	cancelFunc, exists := svc.JobCtxCancels[resp.JobId]
	require.True(t, exists)
	require.NotNil(t, cancelFunc)

	stream := NewMockStream()
	req := &actservice.JobLogRequest{
		JobId:      resp.JobId,
		LastOffset: 0,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.JobLogStream(req, stream)
	}()
	messageCount := 0
loop:
	for {
		select {
		case msg := <-stream.messages:
			t.Logf("[%s] %s", msg.Type.String(), msg.Line)
			require.NotEmpty(t, msg.Line)
			messageCount++
			if messageCount >= 5 {
				break loop
			}
		case <-time.After(10 * time.Second):
			require.Fail(t, "timeout waiting for log messages before cancel")
			break loop
		}
	}

	cancelResp, err := svc.CancelActJob(context.Background(), &actservice.CancelJob{
		JobId: resp.JobId,
	})
	require.NoError(t, err)
	require.NotNil(t, cancelResp)
	t.Logf("Cancel response: %s", cancelResp.Status)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for stream to finish after cancel")
	}

	require.Eventually(t, func() bool {
		_, stillExists := svc.JobCtxCancels[resp.JobId]
		return !stillExists
	}, 20*time.Second, 100*time.Millisecond, "jobCtxCancels map should not contain canceled job")

	logFilePath := filepath.Join(testConf["LOG_FILE_STORAGE"], resp.JobId)
	_ = os.Remove(logFilePath)
}
