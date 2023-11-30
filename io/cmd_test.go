package io

import (
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

var matchAllRegexp = regexp.MustCompile(".*")
var errParsing = errors.New("parsing error")

func TestRunCmdWithOutputParser(t *testing.T) {
	config := NewCommand("go", "", []string{"version"})
	parserCalled := false
	stdout, stderr, exitOk, err := RunCmdWithOutputParser(config, false, &CmdOutputPattern{
		RegExp: matchAllRegexp,
		ExecFunc: func(pattern *CmdOutputPattern) (string, error) {
			parserCalled = true
			return pattern.Line, nil
		},
	})
	assert.NoError(t, err)
	assert.True(t, parserCalled)
	assert.True(t, exitOk)
	assert.Contains(t, stdout, "go version")
	assert.Empty(t, stderr)
}

func TestRunCmdWithOutputParserError(t *testing.T) {
	config := NewCommand("go", "", []string{"version"})
	_, _, exitOk, err := RunCmdWithOutputParser(config, false, &CmdOutputPattern{
		RegExp:   matchAllRegexp,
		ExecFunc: func(pattern *CmdOutputPattern) (string, error) { return pattern.Line, errParsing },
	})
	assert.ErrorContains(t, err, "parsing error")
	assert.False(t, exitOk)
}

var processLineCases = []struct {
	name              string
	cmdOutputPatterns []*CmdOutputPattern
	line              string
	expectedOutput    string
	expectError       bool
}{
	{"Empty", []*CmdOutputPattern{}, "", "", false},

	{"Simple", []*CmdOutputPattern{{
		RegExp:   matchAllRegexp,
		ExecFunc: func(pattern *CmdOutputPattern) (string, error) { return pattern.Line, nil },
	}}, "hello", "hello", false},

	{"Append character once", []*CmdOutputPattern{{
		RegExp:   matchAllRegexp,
		ExecFunc: func(pattern *CmdOutputPattern) (string, error) { return pattern.Line[1:], nil },
	}}, "hello", "ello", false},

	{"Append character twice", []*CmdOutputPattern{
		{
			RegExp:   matchAllRegexp,
			ExecFunc: func(pattern *CmdOutputPattern) (string, error) { return pattern.Line + "l", nil },
		},
		{
			RegExp:   matchAllRegexp,
			ExecFunc: func(pattern *CmdOutputPattern) (string, error) { return pattern.Line + "o", nil },
		},
	}, "hel", "hello", false},

	{"Doesn't match", []*CmdOutputPattern{
		{
			RegExp:   regexp.MustCompile("doesn't match"),
			ExecFunc: func(pattern *CmdOutputPattern) (string, error) { return pattern.Line + "aaaaaa", nil },
		},
		{
			RegExp:   matchAllRegexp,
			ExecFunc: func(pattern *CmdOutputPattern) (string, error) { return pattern.Line + "o", nil },
		},
	}, "hell", "hello", false},

	{"Parsing error", []*CmdOutputPattern{{
		RegExp:   matchAllRegexp,
		ExecFunc: func(pattern *CmdOutputPattern) (string, error) { return "", errParsing },
	}}, "hello", "", true},
}

func TestProcessLine(t *testing.T) {
	for _, testCase := range processLineCases {
		t.Run(testCase.name, func(t *testing.T) {
			errChan := make(chan error, 1)
			defer close(errChan)
			processedLine, hasErrors := processLine(testCase.cmdOutputPatterns, testCase.line, errChan)
			if testCase.expectError {
				assert.True(t, hasErrors)
				assert.ErrorIs(t, errParsing, <-errChan)
			} else {
				assert.False(t, hasErrors)
				assert.Equal(t, testCase.expectedOutput, processedLine)
			}
		})
	}
}
