package actCmd

type CommandOutput interface {
	AddOutput(line []byte, type_ ProcessOutType)
	SetExitCode(val int)

	GetExitCode() chan int
	ProgramError() chan error
	GetOutputChan() chan SingleOutput

	Close()
}
