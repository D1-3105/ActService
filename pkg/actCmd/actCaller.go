package actCmd

import (
	"bufio"
	"context"
	"fmt"
	"github.com/D1-3105/ActService/conf"
	"github.com/golang/glog"
	"io"
	"os"
	"os/exec"
)

type ActCommand struct {
	env            *conf.ActEnviron
	callSubCommand string
	cwd            string
}

func NewActCommand(env *conf.ActEnviron, callSubCommand string, workingDir string) *ActCommand {
	return &ActCommand{
		env:            env,
		callSubCommand: callSubCommand,
		cwd:            workingDir,
	}
}

func (a *ActCommand) getExportString() string {
	return fmt.Sprintf("DOCKER_HOST=%s", a.env.DockerContextPath)
}

func (a *ActCommand) Call(ctx context.Context) (CommandOutput, error) {
	glog.V(1).Infof("Act Command Call: >>%s<< in %s", a.env.ActBinaryPath+" "+a.callSubCommand, a.cwd)
	cmd := exec.CommandContext(ctx, a.env.ActBinaryPath, a.callSubCommand)
	cmd.Env = append(os.Environ(), a.getExportString())
	cmd.Dir = a.cwd

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	glog.V(1).Infof("Started new process: %d", cmd.Process.Pid)

	actOutput := NewActOutput(ctx, cmd)

	readPipe := func(ctx context.Context, pipe io.Reader, outType ProcessOutType) {

		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			actOutput.AddOutput(ctx, scanner.Bytes(), outType)
		}
		if err := scanner.Err(); err != nil {
			glog.Errorf("Error scanning output: %s", err)
		}
	}
	go func(ctx context.Context) {
		defer actOutput.Close()
		go readPipe(ctx, stdout, StdOut)
		go readPipe(ctx, stderr, StdErr)
		select {
		case <-ctx.Done():
			return
		}
	}(ctx)

	return actOutput, nil
}
