package rpc

import (
	"context"
	"errors"
	"fmt"
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
	"slices"
	"time"
)

type ActService struct {
	actservice.UnimplementedActServiceServer
	Schedule          chan interface{}
	JobCtxCancels     map[string]context.CancelFunc
	FileListenersPool *ActService_listen_file.LogFileListeners
}

func NewActService() actservice.ActServiceServer {
	jobCtxList := make(map[string]context.CancelFunc)
	return &ActService{
		JobCtxCancels:     jobCtxList,
		FileListenersPool: ActService_listen_file.NewFileListeners(),
		Schedule:          make(chan interface{}, 1),
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

func (service *ActService) ScheduleActJob(_ context.Context, job *actservice.Job) (*actservice.JobResponse, error) {
	jobUid := uuid.New().String()
	var actEnv conf.ActEnviron
	conf.NewEnviron(&actEnv)

	glog.V(1).Infof("ActService.ScheduleActJob: job uid: %s", jobUid)
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
	var callArgs []string
	if job.WorkflowFile != nil && *job.WorkflowFile != "" {
		callArgs = append(callArgs, "-W", *job.WorkflowFile)
	}
	for _, customFlag := range job.ExtraFlags {
		callArgs = append(callArgs, customFlag)
	}

	if !slices.Contains(callArgs, "-P") {
		callArgs = append(callArgs, "-P", "ubuntu-latest=node:16-buster")
	}
	runIdAdded := false
	for idx, argument := range callArgs {
		if argument == "--container-options" {
			callArgs[idx+1] = fmt.Sprintf("-e RUN_ID=%s %s", jobUid, callArgs[idx+1])
			runIdAdded = true
			break
		}
	}
	if !runIdAdded {
		callArgs = append(callArgs, "--container-options", fmt.Sprintf("-e RUN_ID=%s", jobUid))
	}
	actCommand := actCmd.NewActCommand(
		&actEnv, callArgs, cloned.Path,
	)
	jobContext := context.Background()
	jobContext, service.JobCtxCancels[jobUid] = context.WithCancel(jobContext)
	glog.Infof("ActService.ScheduleActJob: context of job %s, acquiring Schedule lock", jobUid)
	select {
	case service.Schedule <- struct{}{}:
		glog.Infof("ActService.ScheduleActJob: context of job %s, acquired Schedule lock", jobUid)
		break
	case <-time.After(time.Duration(actEnv.JobTimeout) * time.Second):
		glog.Errorf("ActService.ScheduleActJob: context of job %s, timeout waiting for Schedule lock", jobUid)
		break
	}

	output, err := actCommand.Call(jobContext)
	if err != nil {
		defer dispose()
		defer func(jobFile *os.File) {
			_ = jobFile.Close()
		}(jobFile)
		return nil, err
	}

	go ActService_listen_job.ListenJob(
		jobContext, output, jobFile, jobUid,
		// finalizer
		func() {
			defer dispose()
			defer delete(service.JobCtxCancels, jobUid)
			defer func() {
				select {
				case <-service.Schedule:
					break
				case <-time.After(2 * time.Second):
					break
				}
				glog.Infof("ActService.ScheduleActJob: context of job %s, released Schedule lock", jobUid)
			}()
			defer service.JobCtxCancels[jobUid]()
			defer func(fileListenersCtx *ActService_listen_file.LogFileListeners, id string) {
				err := fileListenersCtx.JobDone(id)
				glog.V(1).Infof("ActService.ScheduleActJob: JobDone: id: %s, err: %v", id, err)
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
	glog.Infof("New job stream requested: %s", request.JobId)
	jobFile, err := logFile(request.JobId, os.O_RDONLY)
	if err != nil {
		return err
	}
	glog.V(1).Infof("Listening to %s", jobFile.Name())
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
			glog.V(2).Infof("ActService.JobLogStream[%s]: Send: %v", request.JobId, msg)
		case <-time.After(5 * time.Minute):
			glog.V(1).Infof("ActService.JobLogStream[%s]: timeout waiting for messages", request.JobId)
			return nil
		}
	}
}

func (service *ActService) CancelActJob(
	_ context.Context, cancelJob *actservice.CancelJob,
) (*actservice.CancelJobResult, error) {
	jobCtx, ok := service.JobCtxCancels[cancelJob.JobId]
	if !ok {
		return nil, errors.New(fmt.Sprintf("CancelJob: Job %s not found", cancelJob.JobId))
	}
	jobCtx()
	return &actservice.CancelJobResult{
		Status: fmt.Sprintf("Cancelled job %s", cancelJob.JobId),
	}, nil
}
