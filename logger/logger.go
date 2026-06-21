package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Level represents the severity of a log message.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "debug",
	LevelInfo:  "info",
	LevelWarn:  "warn",
	LevelError: "error",
}

// Logger provides leveled logging with optional JSON output.
type Logger struct {
	level  Level
	json   bool
	stdlog *log.Logger
}

// New creates a new Logger. Set json to true for structured JSON output.
func New(level Level, json bool) *Logger {
	return &Logger{
		level:  level,
		json:   json,
		stdlog: log.New(os.Stderr, "", 0),
	}
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if l.json {
		entry := map[string]interface{}{
			"time":  time.Now().Format(time.RFC3339),
			"level": levelNames[level],
			"msg":   msg,
		}
		b, _ := json.Marshal(entry)
		l.stdlog.Println(string(b))
	} else {
		l.stdlog.Printf("[%s] %s", strings.ToUpper(levelNames[level]), msg)
	}
}

// Debug logs a message at debug level.
func (l *Logger) Debug(format string, args ...interface{}) { l.log(LevelDebug, format, args...) }

// Info logs a message at info level.
func (l *Logger) Info(format string, args ...interface{}) { l.log(LevelInfo, format, args...) }

// Warn logs a message at warn level.
func (l *Logger) Warn(format string, args ...interface{}) { l.log(LevelWarn, format, args...) }

// Error logs a message at error level.
func (l *Logger) Error(format string, args ...interface{}) { l.log(LevelError, format, args...) }

// Fatal logs a message at error level then exits with status 1.
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
	os.Exit(1)
}

// Default is the package-level logger used by the application.
var Default = New(LevelInfo, false)
