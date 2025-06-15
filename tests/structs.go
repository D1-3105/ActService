package tests

import (
	"context"
	"errors"
	"fmt"
	"github.com/D1-3105/ActService/pkg/actCmd"
	"github.com/golang/glog"
	"time"
)

type DummyJobOutput struct {
	getOutputSuccessful bool
	shouldRaiseError    bool

	outputChan        chan actCmd.SingleOutput
	exitCodeChan      chan int
	programmaticError chan error
}

func (d *DummyJobOutput) ProgramError() chan error {
	return d.programmaticError
}

func (d *DummyJobOutput) AddOutput(line []byte, type_ actCmd.ProcessOutType) {
	//TODO implement me
	panic("implement me")
}

func (d *DummyJobOutput) SetExitCode(val int) {
	//TODO implement me
	panic("implement me")
}

func (d *DummyJobOutput) GetExitCode() chan int {
	return d.exitCodeChan
}

func (d *DummyJobOutput) GetOutputChan() chan actCmd.SingleOutput {
	return d.outputChan
}

func (d *DummyJobOutput) Close() {
	close(d.exitCodeChan)
	close(d.outputChan)
	close(d.programmaticError)
}

func SuccessfulDummyJobOutput() *DummyJobOutput {
	return &DummyJobOutput{
		getOutputSuccessful: true,
		outputChan:          make(chan actCmd.SingleOutput),
		exitCodeChan:        make(chan int),
		programmaticError:   make(chan error),
		shouldRaiseError:    false,
	}
}

func DummyEmulator(_ context.Context, dummy *DummyJobOutput) {
	if dummy.getOutputSuccessful {
		for i := 0; i < 5; i++ {
			output := actCmd.SingleOutput{T: actCmd.StdOut, Time: time.Now()}
			output.SetLine([]byte(fmt.Sprintf("line %d", i+1)))
			glog.Info("Input output:", output)
			dummy.outputChan <- output

			output2 := actCmd.SingleOutput{T: actCmd.StdErr, Time: time.Now()}
			output2.SetLine([]byte(fmt.Sprintf("line %d", i+1)))
			glog.Info("Input output2:", output2)
			dummy.outputChan <- output2
			time.Sleep(1 * time.Second)
		}
		dummy.exitCodeChan <- 0
	}
	if dummy.shouldRaiseError {
		dummy.programmaticError <- errors.New("dummy error")
		dummy.exitCodeChan <- 1
	}
}
