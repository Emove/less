package writer

import (
	"github.com/emove/less/pkg/io"
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

func Test_writer_Write(t *testing.T) {
	content := []byte("hello server")

	d := &testWriter{}
	writer := NewBufferWriter(d).(*writer)
	write, err := writer.Write(content)
	if err != nil {
		t.Fatalf("client writer write msg error: %s", err.Error())
	}
	if write != len(content) {
		t.Fatalf("client writer write msg len error, want: %d, got: %d", len(content), write)
	}

	_ = writer.Flush()

	if !reflect.DeepEqual(d.buf, content) {
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
	if bufferWriter.MallocLength() != 11+len(content) {
		t.Fatalf("melloc length error, excepted: %d, got: %d", 11+len(content), bufferWriter.MallocLength())
	}
}

func Test_writer_Malloc(t *testing.T) {
	content := []byte("hello world")

	decorator := &testWriter{}
	bufferWriter := NewBufferWriter(decorator).(*writer)
	doMalloc := func() {
		malloc, _ := bufferWriter.Malloc(len(content))
		copy(malloc, content)
		if !reflect.DeepEqual(bufferWriter.buff[bufferWriter.preWriteIndex:bufferWriter.writeIndex], content) {
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

	malloc, _ := bufferWriter.Malloc(len(content[6:]))
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

	malloc, _ := bufferWriter.Malloc(len(content[6:]))
	copy(malloc, content[6:])

	bufferWriter.Release()

	if bufferWriter.buff != nil ||
		bufferWriter.writeIndex != 0 ||
		bufferWriter.decorator != nil {
		t.Fatalf("release error: %#v", bufferWriter)
	}

}

func TestNewBufferWriterWithBuff(t *testing.T) {
	type args struct {
		buf []byte
	}
	tests := []struct {
		args    args
		call    func(w io.Writer) (interface{}, error)
		want    interface{}
		wantErr bool
	}{
		{
			args: args{buf: make([]byte, 8)},
			call: func(w io.Writer) (interface{}, error) {
				n, err := w.Write([]byte("12345678"))
				return n, err
			},
			want:    8,
			wantErr: false,
		},
		{
			args: args{buf: make([]byte, 8)},
			call: func(w io.Writer) (interface{}, error) {
				return w.Malloc(8)
			},
			want:    make([]byte, 8),
			wantErr: false,
		},
		{
			args: args{buf: make([]byte, 8)},
			call: func(w io.Writer) (interface{}, error) {
				return w.MallocLength(), nil
			},
			want:    0,
			wantErr: false,
		},
		{
			args: args{buf: make([]byte, 8)},
			call: func(w io.Writer) (interface{}, error) {
				if _, err := w.Write([]byte("1234")); err != nil {
					return nil, err
				}
				return w.MallocLength(), nil
			},
			want:    4,
			wantErr: false,
		},
		{
			args: args{buf: make([]byte, 8)},
			call: func(w io.Writer) (interface{}, error) {
				return w.Write([]byte("123456789"))
			},
			want:    0,
			wantErr: true,
		},
		{
			args: args{buf: make([]byte, 8)},
			call: func(w io.Writer) (interface{}, error) {
				malloc, err := w.Malloc(9)
				if len(malloc) == 0 {
					return nil, err
				}
				return malloc, err
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			w := NewBufferWriterWithBuff(tt.args.buf)
			var got interface{}
			var err error
			if got, err = tt.call(w); (err != nil) != tt.wantErr {
				t.Fatalf("Encode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("want: %v, got: %v", tt.want, got)
			}
		})
	}
}
