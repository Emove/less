package log

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
)

var (
	ErrKvsNotInPaired = errors.New("kvs must appear in pairs")
	ErrContextIsNil   = errors.New("context must be non-nil")
)

var (
	defaultLogger, _ = With(NewStdLogger(log.Writer()), "ts", DefaultTimestamp, "caller", DefaultCaller)
	DefaultMsgKey    = "msg"
)

// Logger defines logger interface
// inspired by https://github.com/go-kratos/kratos/blob/main/log
type Logger interface {
	Log(level Level, kvs ...interface{})
}

type FullLogger interface {
	Logger

	Debug(v ...interface{})
	Debugf(format string, v ...interface{})
	Debugw(kvs ...interface{})

	Info(v ...interface{})
	Infof(format string, v ...interface{})
	Infow(kvs ...interface{})

	Warn(v ...interface{})
	Warnf(format string, v ...interface{})
	Warnw(kvs ...interface{})

	Error(v ...interface{})
	Errorf(format string, v ...interface{})
	Errorw(kvs ...interface{})

	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})
	Fatalw(kvs ...interface{})
}

var _ Logger = (*logger)(nil)

type logger struct {
	l              Logger
	ctx            context.Context
	prefixes       []interface{}
	containsValuer bool
}

// Log implements Logger
func (l logger) Log(level Level, kvs ...interface{}) {
	// filter not logging level
	if _, ok := filterLevels[level]; ok {
		return
	}
	keyvals := make([]interface{}, 0, len(l.prefixes)+len(kvs))

	keyvals = append(keyvals, l.prefixes...)
	if l.containsValuer {
		calculateValues(l.ctx, keyvals)
	}
	keyvals = append(keyvals, kvs...)
	l.l.Log(level, keyvals...)
}

// With with default logger fields.
func With(l Logger, kvs ...interface{}) (Logger, error) {
	if len(kvs)&1 != 0 {
		return l, ErrKvsNotInPaired
	}
	d, ok := l.(*logger)
	if !ok {
		return &logger{
			l:              l,
			ctx:            context.Background(),
			prefixes:       kvs,
			containsValuer: containsValuer(kvs),
		}, nil
	}

	prefix := make([]interface{}, 0, len(d.prefixes)+len(kvs))
	prefix = append(prefix, d.prefixes...)
	prefix = append(prefix, kvs...)

	return &logger{
		l:              d.l,
		ctx:            d.ctx,
		prefixes:       prefix,
		containsValuer: d.containsValuer || containsValuer(kvs),
	}, nil
}

// WithContext returns a shallow copy of mockLogger with its context changed
// to ctx. The provided ctx must be non-nil.
func WithContext(ctx context.Context, l Logger) (Logger, error) {
	if nil == ctx {
		return l, ErrContextIsNil
	}
	d, ok := l.(*logger)
	if !ok {
		return &logger{l: l, ctx: ctx}, nil
	}
	return &logger{
		l:              d.l,
		ctx:            ctx,
		prefixes:       d.prefixes,
		containsValuer: d.containsValuer,
	}, nil
}

var _ FullLogger = (*fullLogger)(nil)

type fullLogger struct {
	l *logger
}

func (l *fullLogger) Log(level Level, kvs ...interface{}) {
	l.l.Log(level, kvs...)
}

func (l *fullLogger) Debug(v ...interface{}) {
	l.l.Log(LevelDebug, DefaultMsgKey, fmt.Sprint(v...))
}

func (l *fullLogger) Debugf(format string, v ...interface{}) {
	l.l.Log(LevelDebug, DefaultMsgKey, fmt.Sprintf(format, v...))
}

func (l *fullLogger) Debugw(kvs ...interface{}) {
	l.l.Log(LevelDebug, kvs...)
}

func (l *fullLogger) Info(v ...interface{}) {
	l.l.Log(LevelInfo, DefaultMsgKey, fmt.Sprint(v...))
}

func (l *fullLogger) Infof(format string, v ...interface{}) {
	l.l.Log(LevelInfo, DefaultMsgKey, fmt.Sprintf(format, v...))
}

func (l *fullLogger) Infow(kvs ...interface{}) {
	l.l.Log(LevelInfo, kvs...)
}

func (l *fullLogger) Warn(v ...interface{}) {
	l.l.Log(LevelWarn, DefaultMsgKey, fmt.Sprint(v...))
}

func (l *fullLogger) Warnf(format string, v ...interface{}) {
	l.l.Log(LevelWarn, DefaultMsgKey, fmt.Sprintf(format, v...))
}

func (l *fullLogger) Warnw(kvs ...interface{}) {
	l.l.Log(LevelWarn, kvs...)
}

func (l *fullLogger) Error(v ...interface{}) {
	l.l.Log(LevelError, DefaultMsgKey, fmt.Sprint(v...))
}

func (l *fullLogger) Errorf(format string, v ...interface{}) {
	l.l.Log(LevelError, DefaultMsgKey, fmt.Sprintf(format, v...))
}

func (l *fullLogger) Errorw(kvs ...interface{}) {
	l.l.Log(LevelError, kvs...)
}

func (l *fullLogger) Fatal(v ...interface{}) {
	l.l.Log(LevelFatal, DefaultMsgKey, fmt.Sprint(v...))
	os.Exit(1)
}

func (l *fullLogger) Fatalf(format string, v ...interface{}) {
	l.l.Log(LevelFatal, DefaultMsgKey, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func (l *fullLogger) Fatalw(kvs ...interface{}) {
	l.l.Log(LevelFatal, kvs...)
	os.Exit(1)
}

// WithFullLogger with default logger fields.
// the fields only effects on FullLogger
func WithFullLogger(l FullLogger, kvs ...interface{}) (FullLogger, error) {
	fl, ok := l.(*fullLogger)
	if !ok {
		return l, errors.New("not support")
	}
	d, err := With(fl.l, kvs...)
	if err != nil {
		return l, err
	}
	fl.l = d.(*logger)
	return fl, nil
}
