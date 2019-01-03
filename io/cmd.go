package io

import (
	"bufio"
	"errors"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
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
func RunCmdWithOutputParser(config CmdConfig, regExpStruct ...*CmdOutputPattern) error {
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

	cmdReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	defer cmdReader.Close()
	scanner := bufio.NewScanner(cmdReader)

	err = cmd.Start()
	if err != nil {
		return err
	}

	for scanner.Scan() {
		line := scanner.Text()
		for _, regExp := range regExpStruct {
			regExp.matchedResult = regExp.RegExp.FindString(line)
			if regExp.matchedResult != "" {
				regExp.line = line
				line, err = regExp.ExecFunc()
				if err != nil {
					return err
				}
			}
		}
		log.Output(line)
	}
	if scanner.Err() != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
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

func (reg *CmdOutputPattern) ErrorOnNotFound() (string, error) {
	log.Output(reg.line)
	return "", errors.New("404 Not Found")
}

// RegExp - The regexp that the line will be searched upon.
// matchedResult - The result string that was found by the regex
// line - The output line from the external process
// ExecFunc - The function to execute
type CmdOutputPattern struct {
	RegExp        *regexp.Regexp
	matchedResult string
	line          string
	ExecFunc      func() (string, error)
}