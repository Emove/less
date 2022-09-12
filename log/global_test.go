package log

import (
	"context"
	"testing"
)

func TestDebug(t *testing.T) {
	type args struct {
		v []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestDebugf(t *testing.T) {
	type args struct {
		format string
		v      []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestDebugw(t *testing.T) {
	type args struct {
		kvs []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestError(t *testing.T) {
	type args struct {
		v []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestErrorf(t *testing.T) {
	type args struct {
		format string
		v      []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestErrorw(t *testing.T) {
	type args struct {
		kvs []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestFatal(t *testing.T) {
	type args struct {
		v []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestFatalf(t *testing.T) {
	type args struct {
		format string
		v      []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestFatalw(t *testing.T) {
	type args struct {
		kvs []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestInfo(t *testing.T) {
	type args struct {
		v []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestInfof(t *testing.T) {
	type args struct {
		format string
		v      []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestInfow(t *testing.T) {
	type args struct {
		kvs []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestLog(t *testing.T) {
	type args struct {
		level Level
		kvs   []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestNewContextLogger(t *testing.T) {
	logger := Logger(&l{t: t})
	logger, err := With(logger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	if err != nil {
		t.Fatalf("expect: %v, got: %v", nil, err)
	}
	type count struct {
		cnt int
	}
	type ctxCntKey struct{}
	ctx := context.WithValue(context.Background(), ctxCntKey{}, &count{cnt: 0})
	logger, _ = WithContext(ctx, logger)
	logger, _ = With(logger, "count", Valuer(func(ctx context.Context) interface{} {
		cnt := ctx.Value(ctxCntKey{}).(*count)
		cnt.cnt++
		return cnt.cnt
	}))
	logger.Log(LevelDebug, "msg", "test1")
	logger.Log(LevelInfo, "msg", "test2")
	logger.Log(LevelError, "msg", "test3")

	SetLogger(logger)
	nc := context.WithValue(context.Background(), ctxCntKey{}, &count{cnt: 0})
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

func TestSetLogger(t *testing.T) {

}

func TestWarn(t *testing.T) {
	type args struct {
		v []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestWarnf(t *testing.T) {
	type args struct {
		format string
		v      []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestWarnw(t *testing.T) {
	type args struct {
		kvs []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}
