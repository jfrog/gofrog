package io

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

// Executes an external process and returns its output.
// If the returned output is not needed, use the RunCmd function instead , for better performance.
func RunCmdOutput(config CmdConfig) (string, error) {
	for k, v := range config.GetEnv() {
		if err := os.Setenv(k, v); err != nil {
			return "", err
		}
	}
	cmd := config.GetCmd()
	if config.GetErrWriter() == nil {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = config.GetErrWriter()
		defer config.GetErrWriter().Close()
	}
	output, err := cmd.Output()
	return string(output), err
}

// Runs an external process and prints its output to stdout / stderr.
func RunCmd(config CmdConfig) error {
	for k, v := range config.GetEnv() {
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}

	cmd := config.GetCmd()
	if config.GetStdWriter() == nil {
		cmd.Stdout = os.Stdout
	} else {
		cmd.Stdout = config.GetStdWriter()
		defer config.GetStdWriter().Close()
	}

	if config.GetErrWriter() == nil {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = config.GetErrWriter()
		defer config.GetErrWriter().Close()
	}
	err := cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	// If the command fails to run or doesn't complete successfully ExitError is returned.
	// We would like to return a regular error instead of ExitError,
	// because some frameworks (such as codegangsta used by JFrog CLI) automatically exit when this error is returned.
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		err = errors.New(err.Error())
	}

	return err
}

// Executes the command and captures the output.
// Analyze each line to match the provided regex.
// Returns the complete stdout output of the command.
func RunCmdWithOutputParser(config CmdConfig, prompt bool, regExpStruct ...*CmdOutputPattern) (stdOut string, errorOut string, exitOk bool, err error) {
	var wg sync.WaitGroup
	for k, v := range config.GetEnv() {
		if err = os.Setenv(k, v); err != nil {
			return
		}
	}

	cmd := config.GetCmd()
	stdoutReader, stderrReader, err := createCommandReaders(cmd)
	if err != nil {
		return
	}
	if err = cmd.Start(); err != nil {
		return
	}
	errChan := make(chan error)
	stdoutBuilder := strings.Builder{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for stdoutReader.Scan() {
			line, _ := processLine(regExpStruct, stdoutReader.Text(), errChan)
			if prompt {
				fmt.Fprintf(os.Stderr, line+"\n")
			}
			stdoutBuilder.WriteString(line)
			stdoutBuilder.WriteRune('\n')
		}
	}()
	stderrBuilder := strings.Builder{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for stderrReader.Scan() {
			line, hasError := processLine(regExpStruct, stderrReader.Text(), errChan)
			if prompt {
				fmt.Fprintf(os.Stderr, line+"\n")
			}
			stderrBuilder.WriteString(line)
			stderrBuilder.WriteRune('\n')
			if hasError {
				break
			}
		}
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err = range errChan {
		return
	}
	stdOut = stdoutBuilder.String()
	errorOut = stderrBuilder.String()

	err = cmd.Wait()
	if err != nil {
		return
	}
	exitOk = true
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		// The program has exited with an exit code != 0
		exitOk = false
	}
	return
}

// Run all the input regExpStruct array on the input stdout or stderr line.
// If an error occurred, add it to the error channel.
// regExpStruct - Array of command output patterns to process the line
// line - string line from stdout or stderr
// errChan - if an error occurred, add it to this channel
func processLine(regExpStruct []*CmdOutputPattern, line string, errChan chan error) (processedLine string, hasError bool) {
	var err error
	processedLine = line
	for _, regExp := range regExpStruct {
		if !regExp.RegExp.MatchString(processedLine) {
			continue
		}
		results := CmdOutputPattern{
			RegExp:         regExp.RegExp,
			MatchedResults: regExp.RegExp.FindStringSubmatch(processedLine),
			Line:           processedLine,
			ExecFunc:       regExp.ExecFunc,
		}
		processedLine, err = regExp.ExecFunc(&results)
		if err != nil {
			errChan <- err
			hasError = true
			break
		}
	}
	return
}

// Create command stdout and stderr readers.
// The returned readers are automatically closed after the running command exit and shouldn't be closed explicitly.
// cmd - The command to execute
func createCommandReaders(cmd *exec.Cmd) (*bufio.Scanner, *bufio.Scanner, error) {
	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	stderrReader, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	return bufio.NewScanner(stdoutReader), bufio.NewScanner(stderrReader), nil
}

type CmdConfig interface {
	GetCmd() *exec.Cmd
	GetEnv() map[string]string
	GetStdWriter() io.WriteCloser
	GetErrWriter() io.WriteCloser
}

// RegExp - The regexp that the line will be searched upon.
// MatchedResults - The slice result that was found by the regexp
// Line - The output line from the external process
// ExecFunc - The function to execute
type CmdOutputPattern struct {
	RegExp         *regexp.Regexp
	MatchedResults []string
	Line           string
	ExecFunc       func(pattern *CmdOutputPattern) (string, error)
}

type Command struct {
	Executable string
	CmdName    string
	CmdArgs    []string
	Dir        string
	StrWriter  io.WriteCloser
	ErrWriter  io.WriteCloser
}

func NewCommand(executable, cmdName string, cmdArgs []string) *Command {
	return &Command{Executable: executable, CmdName: cmdName, CmdArgs: cmdArgs}
}

func (config *Command) RunWithOutput() (data []byte, err error) {
	cmd := config.GetCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed running command: '%s %s' with error: %s - %s",
			cmd.Dir,
			strings.Join(cmd.Args, " "),
			err.Error(),
			stderr.String(),
		)
	}
	return stdout.Bytes(), nil
}

func (config *Command) GetCmd() (cmd *exec.Cmd) {
	var cmdStr []string
	if config.CmdName != "" {
		cmdStr = append(cmdStr, config.CmdName)
	}
	if len(config.CmdArgs) > 0 {
		cmdStr = append(cmdStr, config.CmdArgs...)
	}
	cmd = exec.Command(config.Executable, cmdStr...)
	cmd.Dir = config.Dir
	return
}

func (config *Command) GetEnv() map[string]string {
	return map[string]string{}
}

func (config *Command) GetStdWriter() io.WriteCloser {
	return config.StrWriter
}

func (config *Command) GetErrWriter() io.WriteCloser {
	return config.ErrWriter
}
