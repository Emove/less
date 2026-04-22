package log

import (
	"bytes"
	"fmt"
	"io"
	stdlog "log"
)

var _ Logger = (*stdLogger)(nil)

type stdLogger struct {
	log *stdlog.Logger
}

func NewStdLogger(w io.Writer) Logger {
	if w == nil {
		w = io.Discard
	}
	return &stdLogger{
		log: stdlog.New(w, "", 0),
	}
}

func (l *stdLogger) Log(level Level, keyvals ...interface{}) {
	if len(keyvals) == 0 {
		return
	}
	if (len(keyvals) & 1) == 1 {
		keyvals = append(keyvals, "KEYVALS UNPAIRED")
	}
	var buf bytes.Buffer
	buf.WriteString(level.String())
	for i := 0; i < len(keyvals); i += 2 {
		_, _ = fmt.Fprintf(&buf, " %v=%v", keyvals[i], keyvals[i+1])
	}
	_ = l.log.Output(4, buf.String())
}
