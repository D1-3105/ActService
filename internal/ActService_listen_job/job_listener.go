package ActService_listen_job

import (
	"context"
	"github.com/D1-3105/ActService/api/gen/ActService"
	"github.com/D1-3105/ActService/internal/proto_utils"
	"github.com/D1-3105/ActService/pkg/actCmd"
	"github.com/golang/glog"
	"io"
)

func tryFlush(w io.Writer) {
	if fw, ok := w.(interface{ Flush() error }); ok {
		if err := fw.Flush(); err != nil {
			glog.Errorf("Flush error: %v", err)
		}
	}
}

func ListenJob(
	ctx context.Context, output actCmd.CommandOutput, jobFile io.Writer, jobUUID string, finalizer func(),
) {
	defer finalizer()

	for {
		select {
		case <-ctx.Done():
			return
		case out := <-output.GetOutputChan():
			m := actservice.JobLogMessage{
				Timestamp: out.Time.Unix(),
				Type:      actservice.JobLogMessage_OutputType(out.T),
				Line:      out.Line(),
			}
			if err := proto_utils.Write(jobFile, &m); err != nil {
				glog.Errorf("Failed to write protobuf message to file: %v; job uuid: %s", err, jobUUID)
			}
			tryFlush(jobFile)
		case programError := <-output.ProgramError():
			if programError != nil {
				glog.Errorf("Program error for job %s: %v", jobUUID, programError)
			}
		case exitCode := <-output.GetExitCode():
			glog.Infof("Job %s exited with code: %d", jobUUID, exitCode)
			return
		}
	}
}
