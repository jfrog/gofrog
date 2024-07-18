package log

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func SetEnvironmentVariableForLogLevel(t *testing.T, level string) {
	assert.NoError(t, os.Setenv(LogLevelEnv, level))
}

func ResetEnvironmentVariableForLogLevel(t *testing.T) {
	assert.NoError(t, os.Unsetenv(LogLevelEnv))
}

func TestLogger_WithDefaultInfoLogLevel_LogsInfoAndAbove(t *testing.T) {
	// Ensure default INFO level
	SetEnvironmentVariableForLogLevel(t, "")
	defer ResetEnvironmentVariableForLogLevel(t)

	logger := NewLogger(getLogLevel())

	assert.Equal(t, INFO, logger.GetLogLevel())
}

func TestLogger_WithEnvironmentVariableSetToDebug_LogsAllLevels(t *testing.T) {
	SetEnvironmentVariableForLogLevel(t, "DEBUG")
	defer ResetEnvironmentVariableForLogLevel(t)

	logger := NewLogger(getLogLevel())

	assert.Equal(t, DEBUG, logger.GetLogLevel())
}

func TestLogger_WithEnvironmentVariableSetToError_LogsOnlyErrors(t *testing.T) {
	SetEnvironmentVariableForLogLevel(t, "ERROR")
	defer ResetEnvironmentVariableForLogLevel(t)

	logger := NewLogger(getLogLevel())

	assert.Equal(t, ERROR, logger.GetLogLevel())
}

func TestLogger_SetLogLevelChangesLogLevelAtRuntime(t *testing.T) {
	logger := NewLogger(INFO)
	logger.SetLogLevel(DEBUG)

	assert.Equal(t, DEBUG, logger.GetLogLevel())
}

func TestLogger_ConcurrentAccessToSetLogLevel_DoesNotPanic(t *testing.T) {
	logger := NewLogger(INFO)

	done := make(chan bool)
	for i := range 10 {
		go func() {
			logger.SetLogLevel(LevelType(i % 4))
			done <- true
		}()
	}

	for range 10 {
		<-done
	}
}
