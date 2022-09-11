package writer

import (
	"reflect"
	"testing"
)

type testWriter struct {
	buf []byte
}

func (w *testWriter) Write(buf []byte) (n int, err error) {
	w.buf = buf
	return len(buf), nil
}

func TestNewBufferWriter(t *testing.T) {
	t.Logf("%#v", NewBufferWriter(&testWriter{}).(*writer))
}

func TestNewBufferWriterWithBuf(t *testing.T) {
	size := 8
	buf := make([]byte, size)
	writer := NewBufferWriterWithBuf(&testWriter{}, buf).(*writer)
	if len(writer.buff) != size {
		t.Fatalf("buffer size error")
	}
}

func Test_writer_Write(t *testing.T) {
	content := []byte("hello server")

	writer := NewBufferWriterWithBuf(&testWriter{}, make([]byte, len(content))).(*writer)
	write, err := writer.Write(content)
	if err != nil {
		t.Fatalf("client writer write msg error: %s", err.Error())
	}
	if write != len(content) {
		t.Fatalf("client writer write msg len error, want: %d, got: %d", len(content), write)
	}

	if !reflect.DeepEqual(writer.buff, content) {
		t.Fatalf("writer write error")
	}
}

func Test_writer_Flush(t *testing.T) {
	content := []byte("hello world")

	decorator := &testWriter{}
	bufferWriter := NewBufferWriter(decorator)
	_, _ = bufferWriter.Write(content)
	err := bufferWriter.Flush()
	if err != nil {
		t.Fatalf("flush error: %s", err.Error())
	}
	if !reflect.DeepEqual(decorator.buf, content) {
		t.Fatalf("writer flush error")
	}

	{
		err = bufferWriter.Flush()
		if err != nil {
			t.Fatalf("flush error: %s", err.Error())
		}
	}

	content = []byte("second msg")
	_, _ = bufferWriter.Write(content)
	err = bufferWriter.Flush()
	if err != nil {
		t.Fatalf("flush error: %s", err.Error())
	}
	if !reflect.DeepEqual(decorator.buf, content) {
		t.Fatalf("writer flush error")
	}
}

func Test_writer_Malloc(t *testing.T) {
	content := []byte("hello world")

	decorator := &testWriter{}
	bufferWriter := NewBufferWriter(decorator).(*writer)
	doMalloc := func() {
		malloc := bufferWriter.Malloc(len(content))
		copy(malloc, content)
		if !reflect.DeepEqual(bufferWriter.buff[:bufferWriter.writeIndex], content) {
			t.Fatalf("malloc buf write error, want: %s, got: %s", string(content), string(bufferWriter.buff[:bufferWriter.writeIndex]))
		}
	}
	doMalloc()
	_ = bufferWriter.Flush()
	doMalloc()
}

func Test_writer_MallocLength(t *testing.T) {
	decorator := &testWriter{}
	bufferWriter := NewBufferWriter(decorator).(*writer)

	content := []byte("hello world")
	_, _ = bufferWriter.Write(content[:6])

	malloc := bufferWriter.Malloc(len(content[6:]))
	copy(malloc, content[6:])

	if len(content) != bufferWriter.MallocLength() {
		t.Fatalf("malloc length error, want: %d, got: %d", len(content), bufferWriter.MallocLength())
	}
}

func Test_writer_Release(t *testing.T) {
	decorator := &testWriter{}
	bufferWriter := NewBufferWriter(decorator).(*writer)

	content := []byte("hello world")
	_, _ = bufferWriter.Write(content[:6])

	malloc := bufferWriter.Malloc(len(content[6:]))
	copy(malloc, content[6:])

	bufferWriter.Release()

	if bufferWriter.buff != nil ||
		bufferWriter.writeIndex != 0 ||
		bufferWriter.decorator != nil {
		t.Fatalf("release error: %#v", bufferWriter)
	}

}
