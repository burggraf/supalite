package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

type logger struct {
	mu     sync.Mutex
	level  Level
	writer io.Writer
}

var (
	globalLogger *logger
	once         sync.Once
)

func init() {
	globalLogger = &logger{
		level:  LevelInfo,
		writer: os.Stdout,
	}
}

func Debug(msg string, args ...interface{}) {
	globalLogger.log(LevelDebug, msg, args...)
}

func Info(msg string, args ...interface{}) {
	globalLogger.log(LevelInfo, msg, args...)
}

func Warn(msg string, args ...interface{}) {
	globalLogger.log(LevelWarn, msg, args...)
}

func Error(msg string, args ...interface{}) {
	globalLogger.log(LevelError, msg, args...)
}

func SetLevel(level Level) {
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()
	globalLogger.level = level
}

func SetWriter(w io.Writer) {
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()
	globalLogger.writer = w
}

func Logger() *log.Logger {
	return log.New(os.Stdout, "", log.LstdFlags)
}

func (l *logger) log(level Level, msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	prefix := ""
	switch level {
	case LevelDebug:
		prefix = "DEBUG"
	case LevelInfo:
		prefix = "INFO"
	case LevelWarn:
		prefix = "WARN"
	case LevelError:
		prefix = "ERROR"
	}

	logMsg := "[" + prefix + "] " + msg
	if len(args) > 0 {
		for _, arg := range args {
			logMsg += " " + fmt.Sprint(arg)
		}
	}

	log.Println(logMsg)
}
