package actCmd

import "context"

type CommandOutput interface {
	AddOutput(ctx context.Context, line []byte, type_ ProcessOutType)
	SetExitCode(val int)

	GetExitCode() chan int
	ProgramError() chan error
	GetOutputChan() chan SingleOutput

	Close()
}
