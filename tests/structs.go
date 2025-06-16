package tests

import (
	"context"
	"errors"
	"fmt"
	actservice "github.com/D1-3105/ActService/api/gen/ActService"
	"github.com/D1-3105/ActService/pkg/actCmd"
	"github.com/golang/glog"
	"google.golang.org/grpc/metadata"
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

func (d *DummyJobOutput) AddOutput(_ context.Context, _ []byte, _ actCmd.ProcessOutType) {
	//TODO implement me
	panic("implement me")
}

func (d *DummyJobOutput) SetExitCode(_ int) {
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
			glog.V(1).Info("Input output:", output)
			dummy.outputChan <- output

			output2 := actCmd.SingleOutput{T: actCmd.StdErr, Time: time.Now()}
			output2.SetLine([]byte(fmt.Sprintf("line %d", i+1)))
			glog.V(1).Info("Input output2:", output2)
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

// grpc stream mocker
type MockStream struct {
	ctx      context.Context
	cancel   context.CancelFunc
	messages chan *actservice.JobLogMessage
}

func (m *MockStream) Send(msg *actservice.JobLogMessage) error {
	m.messages <- msg
	return nil
}

func NewMockStream() *MockStream {
	ctx, cancel := context.WithCancel(context.Background())
	return &MockStream{
		ctx:      ctx,
		cancel:   cancel,
		messages: make(chan *actservice.JobLogMessage, 100),
	}
}

func (m *MockStream) Context() context.Context {
	return m.ctx
}

func (m *MockStream) Close() {
	m.cancel()
}

func (m *MockStream) SetHeader(_ metadata.MD) error  { return nil }
func (m *MockStream) SendHeader(_ metadata.MD) error { return nil }
func (m *MockStream) SetTrailer(_ metadata.MD)       {}
func (m *MockStream) SendMsg(interface{}) error      { return nil }
func (m *MockStream) RecvMsg(interface{}) error      { return nil }
