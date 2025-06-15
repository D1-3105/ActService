package rpc

import (
	"context"
	actservice "github.com/D1-3105/ActService/api/gen/ActService"
	"github.com/D1-3105/ActService/conf"
	"github.com/D1-3105/ActService/internal/ActService_listen_file"
	"github.com/D1-3105/ActService/internal/ActService_listen_job"
	"github.com/D1-3105/ActService/pkg/actCmd"
	"github.com/D1-3105/ActService/pkg/gitCmd"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"os"
	"path/filepath"
)

type ActService struct {
	actservice.UnimplementedActServiceServer
	JobCtxCancels     map[string]context.CancelFunc
	FileListenersPool *ActService_listen_file.LogFileListeners
}

func NewActService() actservice.ActServiceServer {
	jobCtxList := make(map[string]context.CancelFunc)
	return &ActService{
		JobCtxCancels:     jobCtxList,
		FileListenersPool: ActService_listen_file.NewFileListeners(),
	}
}

func (service *ActService) ScheduleActJob(ctx context.Context, job *actservice.Job) (*actservice.JobResponse, error) {
	jobUid := uuid.New().String()
	glog.Infof("ActService.ScheduleActJob: job uid: %s", jobUid)
	gitFolder, err := gitCmd.NewGitFolder(
		&gitCmd.GitRepo{
			Url:      job.RepoUrl,
			CommitId: job.CommitId,
		},
		"/dev/shm/tests",
	)
	if err != nil {
		return nil, err
	}
	cloned, err := gitFolder.Clone()
	dispose := func() {
		_ = cloned.Dispose()
	}
	if err != nil {
		return nil, err
	}
	var actEnv conf.ActEnviron
	conf.NewEnviron(&actEnv)
	actCommand := actCmd.NewActCommand(
		&actEnv, "-P ubuntu-latest=node:16-buster", cloned.Path,
	)
	var storageEnv conf.StorageEnviron
	conf.NewEnviron(&storageEnv)
	absLogFile, err := filepath.Abs(storageEnv.LogFileStorageRoot)
	if err != nil {
		return nil, err
	}
	jobFile, err := os.OpenFile(filepath.Join(absLogFile, jobUid), os.O_CREATE|os.O_WRONLY, 0x777)
	output, err := actCommand.Call(ctx)
	if err != nil {
		defer dispose()
		defer func(jobFile *os.File) {
			_ = jobFile.Close()
		}(jobFile)
		return nil, err
	}
	jobContext := context.Background()
	jobContext, service.JobCtxCancels[jobUid] = context.WithCancel(jobContext)

	go ActService_listen_job.ListenJob(
		jobContext, output, jobFile, jobUid,
		// finalizer
		func() {
			defer dispose()
			defer output.Close()
			defer delete(service.JobCtxCancels, jobUid)
			defer func(fileListenersCtx *ActService_listen_file.LogFileListeners, id string) {
				err := fileListenersCtx.CancelLogListeners(id)
				glog.Infof("ActService.ScheduleActJob: CancelLogListeners: id: %s, err: %v", id, err)
			}(service.FileListenersPool, jobUid)
			_ = jobFile.Close()
		},
	)
	return &actservice.JobResponse{JobId: jobUid}, nil
}
