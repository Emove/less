package log

import (
	"context"
	"testing"
)

func TestValue(t *testing.T) {
	logger := Logger(&l{t: t})
	logger, _ = With(logger, "ts", DefaultTimestamp, "caller", DefaultCaller)
	logger.Log(LevelInfo, "msg", "helloworld")

	logger = Logger(&l{t: t})
	logger, _ = With(logger)
	logger.Log(LevelDebug, "msg", "helloworld")

	var v1 interface{}
	got := Value(context.Background(), v1)
	if got != v1 {
		t.Errorf("Value() = %v, want %v", got, v1)
	}
	var v2 Valuer = func(ctx context.Context) interface{} {
		return 3
	}
	got = Value(context.Background(), v2)
	res := got.(int)
	if res != 3 {
		t.Errorf("Value() = %v, want %v", res, 3)
	}
}
