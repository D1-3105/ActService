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
	"strings"
	"sync"
)

type ActCommand struct {
	env            *conf.ActEnviron
	callSubCommand []string
	cwd            string
}

func NewActCommand(env *conf.ActEnviron, callSubCommand []string, workingDir string) *ActCommand {
	return &ActCommand{
		env:            env,
		callSubCommand: callSubCommand,
		cwd:            workingDir,
	}
}

func (a *ActCommand) getExportString() string {
	val := a.env.DockerContextPath
	if val == "" {
		val = `""`
	}
	return fmt.Sprintf("DOCKER_HOST=%s", val)
}

func (a *ActCommand) Call(ctx context.Context) (CommandOutput, error) {
	callSubCommandString := strings.Join(a.callSubCommand, " ")
	var cmd *exec.Cmd
	if !a.env.DEBUG {
		glog.V(1).Infof("Act Command Call: >>%s<< in %s", a.env.ActBinaryPath+" "+callSubCommandString, a.cwd)
		cmd = exec.CommandContext(ctx, a.env.ActBinaryPath, a.callSubCommand...)
	} else {
		txt := fmt.Sprintf(
			"%s %s | tee /tmp/output.log",
			a.env.ActBinaryPath,
			strings.Join(a.callSubCommand, " "),
		)
		glog.Infof("sh -c \"%s\"", txt)
		cmd = exec.CommandContext(ctx, "sh", "-c", txt)
	}

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
	var wg sync.WaitGroup
	wg.Add(2)
	actOutput := NewActOutput(ctx, cmd, &wg)
	readPipe := func(ctx context.Context, pipe io.Reader, outType ProcessOutType) {
		defer wg.Done()
		reader := bufio.NewReader(pipe)
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				actOutput.AddOutput(ctx, line, outType)
			}
			if err != nil {
				if err != io.EOF {
					glog.Errorf("Error reading output: %s", err)
				}
				break
			}
		}
	}
	go func(ctx context.Context) {
		go readPipe(ctx, stdout, StdOut)
		go readPipe(ctx, stderr, StdErr)
		select {
		case <-ctx.Done():
			return
		}
	}(ctx)

	return actOutput, nil
}
