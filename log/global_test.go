package log

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestNewContextLogger(t *testing.T) {
	logger := Logger(&mockLogger{t: t})
	logger, _ = With(logger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	type ctxCntKey struct{}
	cnt := 0
	ctx := context.WithValue(context.Background(), ctxCntKey{}, &cnt)
	logger, _ = WithContext(ctx, logger)
	logger, _ = With(logger, "count", Valuer(func(ctx context.Context) interface{} {
		cnt := ctx.Value(ctxCntKey{}).(*int)
		*cnt++
		return *cnt
	}))
	logger.Log(LevelDebug, "msg", "test1")
	logger.Log(LevelInfo, "msg", "test2")
	logger.Log(LevelError, "msg", "test3")

	SetLogger(logger)
	cnt1 := 0
	nc := context.WithValue(context.Background(), ctxCntKey{}, &cnt1)
	l := NewContextLogger(nc)
	l.Log(LevelInfo, "msg", "test4")
	l.Debug("test5")
	l.Info("test6")
	l.Warn("test7")
	l.Error("test8")
	l, _ = WithFullLogger(l, "flag", "fullLogger")
	l.Log(LevelInfo, "msg", "test9")
	l.Debug("test10")
	l.Info("test11")
	l.Warn("test12")
	l.Error("test13")

	logger.Log(LevelInfo, "msg", "test14")
}

func TestFilterLevel(t *testing.T) {
	FilterLevel(LevelDebug, LevelWarn, LevelFatal)
	logger := Logger(&mockLogger{t: t})
	logger, _ = With(logger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	logger.Log(LevelDebug, "msg", "debug")
	logger.Log(LevelInfo, "msg", "info")
	logger.Log(LevelWarn, "msg", "warn")
	logger.Log(LevelError, "msg", "error")
	logger.Log(LevelFatal, "msg", "fatal")
}

func TestSetLogger(t *testing.T) {
	logger, _ := With(global, "ts", DefaultTimestamp)
	SetLogger(logger)
	global.Log(LevelInfo, "msg", "setLogger")
}

func TestGetLogger(t *testing.T) {
	logger, _ := With(global, "ts", DefaultTimestamp)
	SetLogger(logger)
	GetLogger().Log(LevelInfo, "msg", "getLogger")
}

func TestGlobalLog(t *testing.T) {
	buff := &bytes.Buffer{}
	SetLogger(NewStdLogger(buff))

	cases := []struct {
		level   Level
		content []interface{}
	}{
		{
			level:   LevelDebug,
			content: []interface{}{"test debug"},
		},
		{
			level:   LevelInfo,
			content: []interface{}{"test info"},
		},
		{
			level:   LevelInfo,
			content: []interface{}{"test %s", "info"},
		},
		{
			level:   LevelWarn,
			content: []interface{}{"test warn"},
		},
		{
			level:   LevelError,
			content: []interface{}{"test error"},
		},
		{
			level:   LevelError,
			content: []interface{}{"test %s", "error"},
		},
	}

	expected := []string{}
	for _, c := range cases {
		msg := fmt.Sprintf(c.content[0].(string), c.content[1:]...)
		switch c.level {
		case LevelDebug:
			Debug(msg)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelDebug.String(), msg))
			Debugf(c.content[0].(string), c.content[1:]...)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelDebug.String(), msg))
			Debugw("log", msg)
			expected = append(expected, fmt.Sprintf("%s log=%s", LevelDebug.String(), msg))
		case LevelInfo:
			Info(msg)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelInfo.String(), msg))
			Infof(c.content[0].(string), c.content[1:]...)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelInfo.String(), msg))
			Infow("log", msg)
			expected = append(expected, fmt.Sprintf("%s log=%s", LevelInfo.String(), msg))
		case LevelWarn:
			Warn(msg)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelWarn.String(), msg))
			Warnf(c.content[0].(string), c.content[1:]...)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelWarn.String(), msg))
			Warnw("log", msg)
			expected = append(expected, fmt.Sprintf("%s log=%s", LevelWarn.String(), msg))
		case LevelError:
			Error(msg)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelError.String(), msg))
			Errorf(c.content[0].(string), c.content[1:]...)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelError.String(), msg))
			Errorw("log", msg)
			expected = append(expected, fmt.Sprintf("%s log=%s", LevelError.String(), msg))
		}
	}

	Log(LevelInfo, defaultMsgKey, "test log")
	expected = append(expected, fmt.Sprintf("%s msg=%s", "INFO", "test log"))
	expected = append(expected, "")

	result := buff.String()
	t.Log(result)

	if result != strings.Join(expected, "\n") {
		t.Errorf("want: \n%s, got: \n%s", strings.Join(expected, "\n"), result)
	}

}
