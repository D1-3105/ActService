package actCmd

import (
	"context"
	"fmt"
	"github.com/golang/glog"
	"os"
	"os/exec"
	"time"
)

type ProcessOutType int

const (
	StdErr ProcessOutType = 1
	StdOut ProcessOutType = 2
)

type SingleOutput struct {
	T    ProcessOutType
	Time time.Time
	line []byte
}

func (s *SingleOutput) FormatRead() string {
	prefix := ""
	switch s.T {
	case StdErr:
		prefix = "STDERR"
		break
	case StdOut:
		prefix = "STDOUT"
	default:
		prefix = "UNKNOWN"
	}
	return fmt.Sprintf("[%s] {%s} %s", prefix, s.Time.Format(time.RFC3339), string(s.line))
}

func (s *SingleOutput) Line() string {
	return string(s.line)
}

func (s *SingleOutput) SetLine(line []byte) {
	s.line = line
}

type ActOutput struct {
	process          *os.Process
	outChan          chan SingleOutput
	exitCode         chan int
	ProgramErrorChan chan error
}

func NewActOutput(ctx context.Context, cmd *exec.Cmd) CommandOutput {
	output := ActOutput{
		outChan:          make(chan SingleOutput, 100),
		exitCode:         make(chan int, 1),
		ProgramErrorChan: make(chan error, 1),
		process:          cmd.Process,
	}
	go func(ctx context.Context) {
		defer close(output.ProgramErrorChan)
		err := cmd.Wait()
		if err != nil {
			output.ProgramErrorChan <- err
		}
		output.exitCode <- cmd.ProcessState.ExitCode()
	}(ctx)
	return &output
}

func (out *ActOutput) ProgramError() chan error { return out.ProgramErrorChan }

func (out *ActOutput) AddOutput(line []byte, type_ ProcessOutType) {
	select {
	case out.outChan <- SingleOutput{
		T:    type_,
		line: line,
		Time: time.Now(),
	}:
		glog.Infof("Process %d - new output from %v", out.process.Pid, type_)
	default:
	}
}

func (out *ActOutput) SetExitCode(val int) {
	select {
	case out.exitCode <- val:
	default:
	}
}

func (out *ActOutput) GetExitCode() chan int {
	return out.exitCode
}

func (out *ActOutput) GetOutputChan() chan SingleOutput { return out.outChan }

func (out *ActOutput) Close() {
	close(out.outChan)
	close(out.exitCode)
}
