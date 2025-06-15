package ActService_listen_job

import (
	"context"
	"github.com/D1-3105/ActService/api/gen/ActService"
	"github.com/D1-3105/ActService/pkg/actCmd"
	"github.com/golang/glog"
	"github.com/sebnyberg/protoio"
	"io"
)

func ListenJob(
	ctx context.Context, output actCmd.CommandOutput, jobFile io.Writer, jobUUID string, finalizer func(),
) {
	defer finalizer()
	writer := protoio.NewWriter(jobFile)

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
			if err := writer.WriteMsg(&m); err != nil {
				glog.Errorf("Failed to write protobuf message to file: %v; job uuid: %s", err, jobUUID)
			}
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
