package shell

import (
	"os/exec"
)

// We prefer to return stderr over the process exit code
type ExitErrorVerbose struct {
	E exec.ExitError
}

func (e ExitErrorVerbose) Error() string {
	if len(e.E.Stderr) != 0 {
		return string(e.E.Stderr)
	}
	return e.E.Error()
}

func Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", ExitErrorVerbose{*exitErr}
		}
		return "", err
	}
	return string(out), nil
}

/*
type RunOutput struct {
	StdOut   string
	StdErr   string
	ExitCode int
}

func Run(prog string, args ...string) RunOutput {
	cmd := exec.Command(prog, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return RunOutput{StdErr: err.Error(), ExitCode: -1}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return RunOutput{StdErr: err.Error(), ExitCode: -1}
	}
	err = cmd.Start()
	if err != nil {
		return RunOutput{StdErr: err.Error(), ExitCode: -1}
	}
	stdoutBytes, _ := io.ReadAll(stdout)
	stderrBytes, _ := io.ReadAll(stderr)
	err = cmd.Wait()
	return RunOutput{StdOut: string(stdoutBytes), StdErr: string(stderrBytes), ExitCode: 0}

}
*/
