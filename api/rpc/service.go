package rpc

import (
	"context"
	actservice "github.com/D1-3105/ActService/api/gen/ActService"
	"github.com/D1-3105/ActService/conf"
	"github.com/D1-3105/ActService/internal/ActService_utils"
	"github.com/D1-3105/ActService/pkg/actCmd"
	"github.com/D1-3105/ActService/pkg/gitCmd"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"os"
	"path/filepath"
)

var env *conf.ActEnviron
var storageEnv *conf.StorageEnviron

type ActService struct {
	actservice.UnimplementedActServiceServer
	jobCtxCancels map[string]context.CancelFunc
}

func NewActService() actservice.ActServiceServer {
	jobCtxList := make(map[string]context.CancelFunc)
	return &ActService{
		jobCtxCancels: jobCtxList,
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
	actCommand := actCmd.NewActCommand(
		conf.NewActEnviron(), "-P ubuntu-latest=node:16-buster", cloned.Path,
	)
	jobFile, err := os.OpenFile(filepath.Join(filepath.Abs(storageEnv.LogFileStorageRoot), jobUid), os.O_CREATE|os.O_WRONLY, 0x777)
	output, err := actCommand.Call(ctx)
	if err != nil {
		defer dispose()
		defer func(jobFile *os.File) {
			_ = jobFile.Close()
		}(jobFile)
		return nil, err
	}
	jobContext := context.Background()
	jobContext, service.jobCtxCancels[jobUid] = context.WithCancel(jobContext)

	go ActService_utils.ListenJob(
		jobContext, output, jobFile, jobUid,
		// finalizer
		func() {
			defer dispose()
			defer output.Close()
			_ = jobFile.Close()
		},
	)
	return &actservice.JobResponse{JobId: jobUid}, nil
}
