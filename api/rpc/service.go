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
	"time"
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

func logFile(jobUUID string, flag int) (*os.File, error) {
	var storageEnv conf.StorageEnviron
	conf.NewEnviron(&storageEnv)
	absLogFile, err := filepath.Abs(storageEnv.LogFileStorageRoot)
	if err != nil {
		return nil, err
	}
	jobFile, err := os.OpenFile(filepath.Join(absLogFile, jobUUID+".protol"), flag, 0x777)
	return jobFile, err
}

func (service *ActService) ScheduleActJob(ctx context.Context, job *actservice.Job) (*actservice.JobResponse, error) {
	jobUid := uuid.New().String()
	var actEnv conf.ActEnviron
	conf.NewEnviron(&actEnv)

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

	jobFile, err := logFile(jobUid, os.O_CREATE|os.O_WRONLY)

	actCommand := actCmd.NewActCommand(
		&actEnv, "-P ubuntu-latest=node:16-buster", cloned.Path,
	)
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
			defer delete(service.JobCtxCancels, jobUid)
			defer func(fileListenersCtx *ActService_listen_file.LogFileListeners, id string) {
				err := fileListenersCtx.JobDone(id)
				glog.Infof("ActService.ScheduleActJob: JobDone: id: %s, err: %v", id, err)
			}(service.FileListenersPool, jobUid)
			_ = jobFile.Close()
		},
	)
	return &actservice.JobResponse{JobId: jobUid}, nil
}

func (service *ActService) JobLogStream(
	request *actservice.JobLogRequest,
	stream actservice.ActService_JobLogStreamServer,
) error {
	jobFile, err := logFile(request.JobId, os.O_RDONLY)
	if err != nil {
		return err
	}
	glog.Infof("Listening to %s", jobFile.Name())
	defer func(jobFile *os.File) {
		_ = jobFile.Close()
	}(jobFile)

	isActive := service.JobCtxCancels[request.JobId] != nil
	exitCause := ActService_listen_file.EndIterCause{
		EndIter:  make(chan bool),
		EndOnEOF: !isActive,
	}

	ctx := stream.Context()
	listenerCtx, cancelListenerCtx := context.WithCancel(ctx)
	defer cancelListenerCtx()

	inProgressListener := ActService_listen_file.NewInProgressListener(&exitCause, cancelListenerCtx)
	finalizer, err := service.FileListenersPool.AddListener(request.JobId, inProgressListener)
	if err != nil {
		return err
	}

	// message chan
	yield := make(chan *actservice.JobLogMessage)

	// file listener
	go func() {
		err := ActService_listen_file.ListenFile(listenerCtx, jobFile, request.LastOffset, &exitCause, yield, finalizer)
		if err != nil {
			glog.Errorf("ActService.JobLogStream[%s]: ListenFile: %v", request.JobId, err)
		}
	}()

	// main stream loop
	for {
		select {
		case <-ctx.Done(): // client closed the stream
			glog.Errorf("ActService.JobLogStream[%s]: context of stream exited", request.JobId)
			return ctx.Err()
		case msg, ok := <-yield:
			if !ok { // channel is closed
				return nil
			}
			if err := stream.Send(msg); err != nil {
				glog.Errorf("ActService.JobLogStream[%s]: Send: %v", request.JobId, err)
				return err
			}
			glog.Warningf("ActService.JobLogStream[%s]: Send: %v", request.JobId, msg)
		case <-time.After(5 * time.Minute):
			glog.Infof("ActService.JobLogStream[%s]: timeout waiting for messages", request.JobId)
			return nil
		}
	}
}
