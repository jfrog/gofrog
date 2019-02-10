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
		os.Setenv(k, v)
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
		os.Setenv(k, v)
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
	return cmd.Wait()
}

// Executes the command and captures the output.
// Analyze each line to match the provided regex.
// Returns the complete stdout output of the command.
func RunCmdWithOutputParser(config CmdConfig, shouldFailOnError bool, regExpStruct ...*CmdOutputPattern) (string, error) {
	var wg sync.WaitGroup
	for k, v := range config.GetEnv() {
		os.Setenv(k, v)
	}

	cmd := config.GetCmd()
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	defer cmdReader.Close()
	scanner := bufio.NewScanner(cmdReader)
	cmdReaderStderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	defer cmdReaderStderr.Close()
	scannerStderr := bufio.NewScanner(cmdReaderStderr)
	err = cmd.Start()
	if err != nil {
		return "", err
	}
	errChan := make(chan error)
	var stdoutOutput string
	wg.Add(1)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
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
			if line != "" {
				fmt.Println(line)
				stdoutOutput += line + "\n"
			}
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		for scannerStderr.Scan() {
			line := scannerStderr.Text()
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

				if shouldFailOnError {
					scannerError = errors.New(line)
					errChan <- scannerError
					break
				}
			}
			if line != "" {
				fmt.Fprintf(os.Stderr, line+"\n")
			}
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

	for err := range errChan {
		return stdoutOutput, err
	}
	return stdoutOutput, nil
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
