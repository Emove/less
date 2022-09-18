package log

import (
	"log"
	"testing"
)

func TestStdLogger(t *testing.T) {
	logger := NewStdLogger(log.Writer())
	logger, _ = With(logger, "caller", DefaultCaller, "ts", DefaultTimestamp)

	logger.Log(LevelInfo, "msg", "test debug")
	logger.Log(LevelInfo, "msg", "test info")
	logger.Log(LevelInfo, "msg", "test warn")
	logger.Log(LevelInfo, "msg", "test error")
	logger.Log(LevelDebug, "singular")

	logger2 := NewStdLogger(log.Writer())
	logger2.Log(LevelDebug)
}
