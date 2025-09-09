package utils

import (
	"log"
	"os"
	"time"
)

type Logger struct {
	*log.Logger
	level LogLevel
}

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

func NewLogger(level string) *Logger {
	var logLevel LogLevel
	switch level {
	case "debug":
		logLevel = DEBUG
	case "info":
		logLevel = INFO
	case "warn":
		logLevel = WARN
	case "error":
		logLevel = ERROR
	default:
		logLevel = INFO
	}

	return &Logger{
		Logger: log.New(os.Stdout, "", log.LstdFlags),
		level:  logLevel,
	}
}

func (l *Logger) Debug(msg string) {
	if l.level <= DEBUG {
		l.Printf("[DEBUG] %s", msg)
	}
}

func (l *Logger) Info(msg string) {
	if l.level <= INFO {
		l.Printf("[INFO] %s", msg)
	}
}

func (l *Logger) Warn(msg string) {
	if l.level <= WARN {
		l.Printf("[WARN] %s", msg)
	}
}

func (l *Logger) Error(msg string) {
	if l.level <= ERROR {
		l.Printf("[ERROR] %s", msg)
	}
}

func GetCurrentTimestamp() int64 {
	return time.Now().Unix()
}
