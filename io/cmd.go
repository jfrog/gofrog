package io

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

// Returns the output of the external process.
// If the output is not needed, use RunCmd
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

// Runs the external process and prints the output of the process.
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
func RunCmdWithOutputParser(config CmdConfig, regExpStruct ...*CmdOutputPattern) (string, error) {
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
					regExp.matchedResults = regExp.RegExp.FindStringSubmatch(line)
					regExp.line = line
					line, err = regExp.ExecFunc()
					if err != nil {
						errChan <- err
					}
				}
			}
			fmt.Println(line)
			stdoutOutput += line + "\n"
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
					regExp.matchedResults = regExp.RegExp.FindStringSubmatch(line)
					regExp.line = line
					line, scannerError = regExp.ExecFunc()
					if scannerError != nil {
						errChan <- scannerError
						break
					}
				}
			}
			fmt.Fprintf(os.Stderr, line+"\n")
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

func GetRegExp(regex string) (*regexp.Regexp, error) {
	regExp, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}

	return regExp, nil
}

// Mask the credentials information from the line. The credentials are build as user:password
// For example: http://user:password@127.0.0.1:8081/artifactory/path/to/repo
func (reg *CmdOutputPattern) MaskCredentials() (string, error) {
	splittedResult := strings.Split(reg.matchedResults[0], "//")
	return strings.Replace(reg.line, reg.matchedResults[0], splittedResult[0]+"//***.***@", 1), nil
}

func (reg *CmdOutputPattern) Error() (string, error) {
	fmt.Fprintf(os.Stderr, reg.line)
	if len(reg.matchedResults) > 1 {
		return "", errors.New(reg.ErrorMessage + strings.TrimSpace(reg.matchedResults[1]))
	}
	return "", errors.New(reg.ErrorMessage)
}

// RegExp - The regexp that the line will be searched upon.
// matchedResults - The slice result that was found by the regex
// line - The output line from the external process
// ErrorMessage - Error message or part of the error that will be returned
// ExecFunc - The function to execute
type CmdOutputPattern struct {
	RegExp         *regexp.Regexp
	matchedResults []string
	line           string
	ErrorMessage   string
	ExecFunc       func() (string, error)
}
