package io

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
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
	if _, ok := err.(*exec.ExitError); ok {
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
	wg.Add(1)
	go func() {
		for stdoutReader.Scan() {
			line := stdoutReader.Text()
			for _, regExp := range regExpStruct {
				matched := regExp.RegExp.Match([]byte(line))
				if matched {
					regExp.MatchedResults = regExp.RegExp.FindStringSubmatch(line)
					regExp.Line = line
					line, err = regExp.ExecFunc(regExp)
					if err != nil {
						errChan <- err
					}
				}
			}
			if prompt {
				fmt.Fprintf(os.Stderr, line+"\n")
			}
			stdOut += line + "\n"
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		for stderrReader.Scan() {
			line := stderrReader.Text()
			var scannerError error
			for _, regExp := range regExpStruct {
				matched := regExp.RegExp.Match([]byte(line))
				if matched {
					regExp.MatchedResults = regExp.RegExp.FindStringSubmatch(line)
					regExp.Line = line
					line, scannerError = regExp.ExecFunc(regExp)
					if scannerError != nil {
						errChan <- scannerError
						break
					}
				}
			}
			if prompt {
				fmt.Fprintf(os.Stderr, line+"\n")
			}
			errorOut += line + "\n"
			if scannerError != nil {
				break
			}
		}
		wg.Done()
	}()

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err = range errChan {
		return
	}

	err = cmd.Wait()
	if err != nil {
		return
	}
	exitOk = true
	if _, ok := err.(*exec.ExitError); ok {
		// The program has exited with an exit code != 0
		exitOk = false
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
// MatchedResults - The slice result that was found by the regex
// Line - The output line from the external process
// ExecFunc - The function to execute
type CmdOutputPattern struct {
	RegExp         *regexp.Regexp
	MatchedResults []string
	Line           string
	ExecFunc       func(pattern *CmdOutputPattern) (string, error)
}
