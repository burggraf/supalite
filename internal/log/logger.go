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

var (
	globalLogger *Logger
	once         sync.Once
)

type Logger struct {
	mu     sync.Mutex
	level  Level
	writer io.Writer
}

func init() {
	globalLogger = &Logger{
		level:  LevelInfo,
		writer: os.Stdout,
	}
}

func SetLevel(level Level) {
	globalLogger.SetLevel(level)
}

func SetWriter(w io.Writer) {
	globalLogger.SetWriter(w)
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) SetWriter(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writer = w
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

func (l *Logger) log(level Level, msg string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

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

	logMsg := fmt.Sprintf("[%s] %s", prefix, msg)
	if len(args) > 0 {
		logMsg += fmt.Sprintf(" %+v", args)
	}
	fmt.Fprintln(l.writer, logMsg)
}

// StandardLogger returns a standard library logger for compatibility
func StandardLogger() *log.Logger {
	return log.New(os.Stdout, "", log.LstdFlags)
}
