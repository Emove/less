package log

import (
	"fmt"
	stdlog "log"
	"os"
)

const DefaultMsgKey = "msg"

// Logger defines logger interface
type Logger interface {
	Log(level Level, kvs ...interface{})
}

var global Logger = NewStdLogger(stdlog.Writer())

func SetLogger(l Logger) {
	if l == nil {
		global = NewStdLogger(stdlog.Writer())
		return
	}
	global = l
}

func GetLogger() Logger {
	return global
}

func Log(level Level, kvs ...interface{}) {
	global.Log(level, kvs...)
}

func Debug(v ...interface{}) {
	global.Log(LevelDebug, DefaultMsgKey, fmt.Sprint(v...))
}

func Debugf(format string, v ...interface{}) {
	global.Log(LevelDebug, DefaultMsgKey, fmt.Sprintf(format, v...))
}

func Debugw(kvs ...interface{}) {
	global.Log(LevelDebug, kvs...)
}

func Info(v ...interface{}) {
	global.Log(LevelInfo, DefaultMsgKey, fmt.Sprint(v...))
}

func Infof(format string, v ...interface{}) {
	global.Log(LevelInfo, DefaultMsgKey, fmt.Sprintf(format, v...))
}

func Infow(kvs ...interface{}) {
	global.Log(LevelInfo, kvs...)
}

func Warn(v ...interface{}) {
	global.Log(LevelWarn, DefaultMsgKey, fmt.Sprint(v...))
}

func Warnf(format string, v ...interface{}) {
	global.Log(LevelWarn, DefaultMsgKey, fmt.Sprintf(format, v...))
}

func Warnw(kvs ...interface{}) {
	global.Log(LevelWarn, kvs...)
}

func Error(v ...interface{}) {
	global.Log(LevelError, DefaultMsgKey, fmt.Sprint(v...))
}

func Errorf(format string, v ...interface{}) {
	global.Log(LevelError, DefaultMsgKey, fmt.Sprintf(format, v...))
}

func Errorw(kvs ...interface{}) {
	global.Log(LevelError, kvs...)
}

func Fatal(v ...interface{}) {
	global.Log(LevelFatal, DefaultMsgKey, fmt.Sprint(v...))
	os.Exit(1)
}

func Fatalf(format string, v ...interface{}) {
	global.Log(LevelFatal, DefaultMsgKey, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func Fatalw(kvs ...interface{}) {
	global.Log(LevelFatal, kvs...)
	os.Exit(1)
}
