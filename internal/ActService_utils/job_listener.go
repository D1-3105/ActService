package ActService_utils

import (
	"context"
	"github.com/D1-3105/ActService/api/gen/ActService"
	"github.com/D1-3105/ActService/pkg/actCmd"
	"github.com/golang/glog"
	"google.golang.org/protobuf/proto"
	"io"
)

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
			outSerialized, err := proto.Marshal(&m)
			if err != nil {
				glog.Errorf("Failed to serialize job output: %v; struct: %v", err, out)
				continue
			}
			_, err = jobFile.Write(outSerialized)
			if err != nil {
				glog.Errorf("Error writing to job >%s< file: %v", jobUUID, err)
				continue
			}
			break
		case programError := <-output.ProgramError():
			if programError != nil {
				glog.Errorf("Error of corresponding job: %v; job uuid: %s;", programError, jobUUID)
			}
			break
		case exitCode := <-output.GetExitCode():
			glog.Infof("Received exit code for job %s: %d", jobUUID, exitCode)
			return
		}
	}
}
