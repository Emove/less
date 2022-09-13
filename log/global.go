package log

import (
	"context"
	"fmt"
	"os"
)

var (
	global       Logger
	filterLevels = make(map[Level]struct{})
)

func init() {
	global = defaultLogger
}

// SetLogger replace default std logger
func SetLogger(l Logger) {
	global = l
}

// GetLogger returns global logger
func GetLogger() Logger {
	return global
}

// FilterLevel sets not logging level
func FilterLevel(level ...Level) {
	for _, l := range level {
		switch l {
		case LevelDebug:
			filterLevels[l] = struct{}{}
		case LevelInfo:
			filterLevels[l] = struct{}{}
		case LevelWarn:
			filterLevels[l] = struct{}{}
		case LevelError:
			filterLevels[l] = struct{}{}
		case LevelFatal:
			filterLevels[l] = struct{}{}
		default:
		}
	}
}

// NewContextLogger returns a FullLogger with context
// and the context only effects on FullLogger
func NewContextLogger(ctx context.Context) FullLogger {
	l, _ := WithContext(ctx, global)
	return &fullLogger{l: l.(*logger)}
}

func Log(level Level, kvs ...interface{}) {
	global.Log(level, kvs...)
}

func Debug(v ...interface{}) {
	global.Log(LevelDebug, defaultMsgKey, fmt.Sprint(v...))
}

func Debugf(format string, v ...interface{}) {
	global.Log(LevelDebug, defaultMsgKey, fmt.Sprintf(format, v...))
}

func Debugw(kvs ...interface{}) {
	global.Log(LevelDebug, kvs...)
}

func Info(v ...interface{}) {
	global.Log(LevelInfo, defaultMsgKey, fmt.Sprint(v...))
}

func Infof(format string, v ...interface{}) {
	global.Log(LevelInfo, defaultMsgKey, fmt.Sprintf(format, v...))
}

func Infow(kvs ...interface{}) {
	global.Log(LevelInfo, kvs...)
}

func Warn(v ...interface{}) {
	global.Log(LevelWarn, defaultMsgKey, fmt.Sprint(v...))
}

func Warnf(format string, v ...interface{}) {
	global.Log(LevelWarn, defaultMsgKey, fmt.Sprintf(format, v...))
}

func Warnw(kvs ...interface{}) {
	global.Log(LevelWarn, kvs...)
}

func Error(v ...interface{}) {
	global.Log(LevelError, defaultMsgKey, fmt.Sprint(v...))
}

func Errorf(format string, v ...interface{}) {
	global.Log(LevelError, defaultMsgKey, fmt.Sprintf(format, v...))
}

func Errorw(kvs ...interface{}) {
	global.Log(LevelError, kvs...)
}

func Fatal(v ...interface{}) {
	global.Log(LevelFatal, defaultMsgKey, fmt.Sprint(v...))
	os.Exit(1)
}

func Fatalf(format string, v ...interface{}) {
	global.Log(LevelFatal, defaultMsgKey, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func Fatalw(kvs ...interface{}) {
	global.Log(LevelFatal, kvs...)
	os.Exit(1)
}
