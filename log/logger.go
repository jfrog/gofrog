package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/gookit/color"
)

const (
	LogLevelEnv = "JFROG_LOG_LEVEL"
)

type Log interface {
	Debug(a ...interface{})
	Info(a ...interface{})
	Warn(a ...interface{})
	Error(a ...interface{})
	Output(a ...interface{})
	GetLogLevel() LevelType
}

var (
	// The logger instance
	_logger *Logger
	// Used to ensure _logger is initialized only once
	once sync.Once
)

func GetLogger() *Logger {
	once.Do(func() {
		_logger = NewLogger(getLogLevel())
	})
	return _logger
}

type LevelType int

const (
	ERROR LevelType = iota
	WARN
	INFO
	DEBUG
)

func getLogLevel() LevelType {
	switch strings.ToUpper(os.Getenv(LogLevelEnv)) {
	case "ERROR":
		return ERROR
	case "WARN":
		return WARN
	case "DEBUG":
		return DEBUG
	default:
		return INFO
	}
}

type Logger struct {
	LogLevel  LevelType
	OutputLog *log.Logger
	DebugLog  *log.Logger
	InfoLog   *log.Logger
	WarnLog   *log.Logger
	ErrorLog  *log.Logger
	// Mutex to protect access to the logger
	mu sync.Mutex
}

func NewLogger(logLevel LevelType) *Logger {
	logger := new(Logger)
	logger.SetLogLevel(logLevel)
	logger.SetOutputWriter()
	logger.SetLogsWriter()
	return logger
}

func (logger *Logger) SetLogLevel(levelEnum LevelType) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.LogLevel = levelEnum
}

func (logger *Logger) SetOutputWriter() {
	logger.OutputLog = log.New(io.Writer(os.Stdout), "", 0)
}

func (logger *Logger) SetLogsWriter() {
	stdErrWriter := io.Writer(os.Stderr)
	logger.DebugLog = log.New(stdErrWriter, getLogPrefix(DEBUG), 0)
	logger.InfoLog = log.New(stdErrWriter, getLogPrefix(INFO), 0)
	logger.WarnLog = log.New(stdErrWriter, getLogPrefix(WARN), 0)
	logger.ErrorLog = log.New(stdErrWriter, getLogPrefix(ERROR), 0)
}

var prefixStyles = map[LevelType]struct {
	logLevel string
	color    color.Color
}{
	DEBUG: {logLevel: "Debug", color: color.Cyan},
	INFO:  {logLevel: "Info", color: color.Blue},
	WARN:  {logLevel: "Warn", color: color.Yellow},
	ERROR: {logLevel: "Error", color: color.Red},
}

func getLogPrefix(logType LevelType) string {
	if logPrefixStyle, ok := prefixStyles[logType]; ok {
		return fmt.Sprintf("[%s] ", logPrefixStyle.logLevel)
	}
	return ""
}

func Debug(a ...interface{}) {
	GetLogger().Debug(a...)
}

func Debugf(format string, a ...interface{}) {
	GetLogger().Debug(fmt.Sprintf(format, a...))
}

func Info(a ...interface{}) {
	GetLogger().Info(a...)
}

func Infof(format string, a ...interface{}) {
	GetLogger().Info(fmt.Sprintf(format, a...))
}

func Warn(a ...interface{}) {
	GetLogger().Warn(a...)
}

func Error(a ...interface{}) {
	GetLogger().Error(a...)
}

func Output(a ...interface{}) {
	GetLogger().Output(a...)
}

func (logger *Logger) GetLogLevel() LevelType {
	return logger.LogLevel
}

func (logger *Logger) Debug(a ...interface{}) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if logger.GetLogLevel() >= DEBUG {
		logger.Println(logger.DebugLog, a...)
	}
}

func (logger *Logger) Info(a ...interface{}) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if logger.GetLogLevel() >= INFO {
		logger.Println(logger.InfoLog, a...)
	}
}

func (logger *Logger) Warn(a ...interface{}) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if logger.GetLogLevel() >= WARN {
		logger.Println(logger.WarnLog, a...)
	}
}

func (logger *Logger) Error(a ...interface{}) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if logger.GetLogLevel() >= ERROR {
		logger.Println(logger.ErrorLog, a...)
	}
}

func (logger *Logger) Output(a ...interface{}) {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	logger.Println(logger.OutputLog, a...)
}

func (logger *Logger) Println(log *log.Logger, values ...interface{}) {
	log.Println(values...)
}
