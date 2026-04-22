package log

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

type mockLogger struct {
	entries []string
}

func (l *mockLogger) Log(level Level, kvs ...interface{}) {
	var builder strings.Builder
	builder.WriteString(level.String())
	for i := 0; i < len(kvs); i += 2 {
		builder.WriteString(fmt.Sprintf(" %v=%v", kvs[i], kvs[i+1]))
	}
	l.entries = append(l.entries, builder.String())
}

func resetLogger(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		SetLogger(NewStdLogger(&bytes.Buffer{}))
	})
}

func TestSetLoggerAndGetLogger(t *testing.T) {
	resetLogger(t)

	logger := &mockLogger{}
	SetLogger(logger)

	if got := GetLogger(); got != logger {
		t.Fatal("GetLogger did not return the logger set by SetLogger")
	}

	Info("hello")
	Debugw("key", "value")

	expected := []string{
		"INFO msg=hello",
		"DEBUG key=value",
	}
	if len(logger.entries) != len(expected) {
		t.Fatalf("got %v, want %v", logger.entries, expected)
	}
	for i := range expected {
		if logger.entries[i] != expected[i] {
			t.Fatalf("entry[%d] = %q, want %q", i, logger.entries[i], expected[i])
		}
	}
}

func TestPackageLoggerOutput(t *testing.T) {
	resetLogger(t)

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

	Log(LevelInfo, DefaultMsgKey, "test log")
	expected = append(expected, fmt.Sprintf("%s msg=%s", LevelInfo.String(), "test log"))

	result := buff.String()
	expected = append(expected, "")
	if result != strings.Join(expected, "\n") {
		t.Errorf("want: \n%s, got: \n%s", strings.Join(expected, "\n"), result)
	}
}

func TestStdLoggerHandlesOddAndEmptyKeyvals(t *testing.T) {
	logger := NewStdLogger(&bytes.Buffer{})
	std := logger.(*stdLogger)

	std.Log(LevelDebug)
	if got := std.log.Writer().(*bytes.Buffer).String(); got != "" {
		t.Fatalf("empty log call wrote %q, want empty", got)
	}

	std.Log(LevelDebug, "singular")
	if got := std.log.Writer().(*bytes.Buffer).String(); got != "DEBUG singular=KEYVALS UNPAIRED\n" {
		t.Fatalf("got %q", got)
	}
}
