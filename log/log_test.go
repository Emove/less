package log

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

type mockLogger struct {
	t *testing.T
}

func (l *mockLogger) Log(level Level, kvs ...interface{}) {
	builder := strings.Builder{}
	builder.WriteString(level.String() + " ")
	for i := 0; i < len(kvs); i += 2 {
		builder.WriteString(fmt.Sprintf("%v=%v ", kvs[i], kvs[i+1]))
	}
	l.t.Log(builder.String())
}

func TestWith(t *testing.T) {
	logger := Logger(&mockLogger{t: t})
	logger, err := With(logger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	if err != nil {
		t.Fatalf("expect: %v, got: %v", nil, err)
	}
	logger.Log(LevelDebug, "msg", "test1")
	logger.Log(LevelInfo, "msg", "test2")
	logger.Log(LevelError, "msg", "test3")

	logger, err = With(logger, "singular")
	if err == nil {
		t.Fatalf("expect: %v, got: %v", ErrKvsNotInPaired, nil)
		return
	}
}

func TestWithContext(t *testing.T) {
	logger := Logger(&mockLogger{t: t})
	cnt := 0
	type ctxCntKey struct{}
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
}

func TestFullLogger(t *testing.T) {
	buff := &bytes.Buffer{}
	SetLogger(NewStdLogger(buff))
	logger := NewContextLogger(context.Background())

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
			logger.Debug(msg)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelDebug.String(), msg))
			logger.Debugf(c.content[0].(string), c.content[1:]...)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelDebug.String(), msg))
			logger.Debugw("log", msg)
			expected = append(expected, fmt.Sprintf("%s log=%s", LevelDebug.String(), msg))
		case LevelInfo:
			logger.Info(msg)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelInfo.String(), msg))
			logger.Infof(c.content[0].(string), c.content[1:]...)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelInfo.String(), msg))
			logger.Infow("log", msg)
			expected = append(expected, fmt.Sprintf("%s log=%s", LevelInfo.String(), msg))
		case LevelWarn:
			logger.Warn(msg)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelWarn.String(), msg))
			logger.Warnf(c.content[0].(string), c.content[1:]...)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelWarn.String(), msg))
			logger.Warnw("log", msg)
			expected = append(expected, fmt.Sprintf("%s log=%s", LevelWarn.String(), msg))
		case LevelError:
			logger.Error(msg)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelError.String(), msg))
			logger.Errorf(c.content[0].(string), c.content[1:]...)
			expected = append(expected, fmt.Sprintf("%s msg=%s", LevelError.String(), msg))
			logger.Errorw("log", msg)
			expected = append(expected, fmt.Sprintf("%s log=%s", LevelError.String(), msg))
		}
	}

	result := buff.String()
	t.Log(result)
	expected = append(expected, "")
	if result != strings.Join(expected, "\n") {
		t.Errorf("want: \n%s, got: \n%s", strings.Join(expected, "\n"), result)
	}

}
