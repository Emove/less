package log

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type l struct {
	t *testing.T
}

func (l *l) Log(level Level, kvs ...interface{}) {
	builder := strings.Builder{}
	builder.WriteString(level.String() + " ")
	for i := 0; i < len(kvs); i += 2 {
		builder.WriteString(fmt.Sprintf("%v=%v ", kvs[i], kvs[i+1]))
	}
	l.t.Log(builder.String())
}

func TestWith(t *testing.T) {
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

	logger, err = With(logger, "singular")
	if err == nil {
		t.Fatalf("expect: %v, got: %v", ErrKvsNotInPaired, nil)
		return
	}
}
