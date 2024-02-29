package signalr_server

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

var slog = atomic.Int32{}

const (
	Debug = iota
	Info
	Warning
	Error
	Fatal
)

func LogDebug(message any) {
	logCore(Debug, message, nil)
}

func LogInfo(message any) {
	logCore(Info, message, nil)
}

func LogWarning(message any, err error) {
	logCore(Warning, message, err)
}

func LogError(message any, err error) {
	logCore(Error, message, err)
}

func LogFatal(message any, err error) {
	logCore(Fatal, message, err)
	os.Exit(-1)
}

func logCore(level int32, message any, err error) {
	if level >= slog.Load() {
		levelStr := logLevelToString(level)
		fmt.Printf("%s %v %v %v\n", levelStr, time.Now(), message, err)
	}
}

func logLevelToString(level int32) string {
	switch level {
	case Debug:
		return "Debug"
	case Info:
		return "Info"
	case Warning:
		return "Warning"
	case Error:
		return "Error"
	case Fatal:
		return "Fatal"
	}
	panic("invalid log level")
}

func SetLogLevel(level int32) {
	slog.Store(level)
}

func init() {
	slog.Store(Info)
}
