package io

import (
	"bufio"
	"errors"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

func RunCmdOutput(config CmdConfig) ([]byte, error) {
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
	return cmd.Output()
}

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
func RunCmdWithOutputParser(config CmdConfig, regExpStruct ...*CmdOutputPattern) (string, error) {
	var wg sync.WaitGroup
	for k, v := range config.GetEnv() {
		os.Setenv(k, v)
	}

	cmd := config.GetCmd()
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	defer cmdReader.Close()
	scanner := bufio.NewScanner(cmdReader)
	cmdReaderStderr, err := cmd.StderrPipe()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	defer cmdReaderStderr.Close()
	scannerStderr := bufio.NewScanner(cmdReaderStderr)
	err = cmd.Start()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	errChan := make(chan error)
	var output string
	wg.Add(1)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			for _, regExp := range regExpStruct {
				regExp.matchedResult = regExp.RegExp.FindString(line)
				if regExp.matchedResult != "" {
					regExp.line = line
					line, err = regExp.ExecFunc()
					if err != nil {
						errChan <- err
					}
				}
			}
			log.Output(line)
			output += line + "\n"
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		for scannerStderr.Scan() {
			line := scannerStderr.Text()
			var scannerError error
			for _, regExp := range regExpStruct {
				regExp.matchedResult = regExp.RegExp.FindString(line)
				if regExp.matchedResult != "" {
					regExp.line = line
					line, scannerError = regExp.ExecFunc()
					if scannerError != nil {
						errChan <- scannerError
						break
					}
				}
			}
			log.Output(line)
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
		return output, err
	}
	return output, nil
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
	splittedResult := strings.Split(reg.matchedResult, "//")
	return strings.Replace(reg.line, reg.matchedResult, splittedResult[0]+"//***.***@", 1), nil
}

func (reg *CmdOutputPattern) Error() (string, error) {
	log.Output(reg.line)
	return "", errorutils.CheckError(errors.New(reg.ErrorMessage + strings.TrimSpace(reg.ModuleRegExp.FindString(reg.line))))
}

// RegExp - The regexp that the line will be searched upon.
// ModuleRegExp - The regex that the module@version will be searched upon
// matchedResult - The result string that was found by the regex
// line - The output line from the external process
// ErrorMessage - Error message or part of the error that will be returned
// ExecFunc - The function to execute
type CmdOutputPattern struct {
	RegExp        *regexp.Regexp
	ModuleRegExp  *regexp.Regexp
	matchedResult string
	line          string
	ErrorMessage  string
	ExecFunc      func() (string, error)
}