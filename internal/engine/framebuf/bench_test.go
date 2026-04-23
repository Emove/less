package framebuf

import (
	"bytes"
	"io"
	"testing"
)

type benchChunkReader struct {
	chunks [][]byte
	index  int
}

func newBenchChunkReader(chunks ...[]byte) *benchChunkReader {
	return &benchChunkReader{chunks: chunks}
}

func (r *benchChunkReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	chunk := r.chunks[r.index]
	r.index++
	return copy(p, chunk), nil
}

func BenchmarkReaderSlice64B(b *testing.B) {
	b.ReportAllocs()

	payload := bytes.Repeat([]byte("a"), 64)
	b.SetBytes(int64(len(payload)))

	for i := 0; i < b.N; i++ {
		rd := NewReader(bytes.NewReader(payload))
		frame, err := rd.Slice(len(payload))
		if err != nil {
			b.Fatal(err)
		}
		frame.Release()
		rd.Release()
	}
}

func BenchmarkReaderSlice1K(b *testing.B) {
	b.ReportAllocs()

	payload := bytes.Repeat([]byte("a"), 1<<10)
	b.SetBytes(int64(len(payload)))

	for i := 0; i < b.N; i++ {
		rd := NewReader(bytes.NewReader(payload))
		frame, err := rd.Slice(len(payload))
		if err != nil {
			b.Fatal(err)
		}
		frame.Release()
		rd.Release()
	}
}

func BenchmarkReaderSlice64K(b *testing.B) {
	b.ReportAllocs()

	payload := bytes.Repeat([]byte("a"), 64<<10)
	b.SetBytes(int64(len(payload)))

	for i := 0; i < b.N; i++ {
		rd := NewReader(bytes.NewReader(payload))
		frame, err := rd.Slice(len(payload))
		if err != nil {
			b.Fatal(err)
		}
		frame.Release()
		rd.Release()
	}
}

func BenchmarkReaderFragmentedPeek(b *testing.B) {
	b.ReportAllocs()

	const size = 64
	payload := bytes.Repeat([]byte("x"), size)
	chunks := make([][]byte, 0, size/8)
	for i := 0; i < size; i += 8 {
		chunks = append(chunks, payload[i:i+8])
	}
	b.SetBytes(size)

	for i := 0; i < b.N; i++ {
		rd := newReader(newBenchChunkReader(chunks...), defaultAllocator, 8)
		p, err := rd.Peek(size)
		if err != nil {
			b.Fatal(err)
		}
		if len(p) != size {
			b.Fatalf("len = %d, want %d", len(p), size)
		}
		rd.Release()
	}
}

func BenchmarkWriterWriteFrame(b *testing.B) {
	b.ReportAllocs()

	payload := NewFrame(bytes.Repeat([]byte("a"), 1<<10))
	defer payload.Release()
	b.SetBytes(1 << 10)

	for i := 0; i < b.N; i++ {
		w := NewWriter()
		if err := w.WriteFrame(payload); err != nil {
			b.Fatal(err)
		}
		w.Release()
	}
}

func BenchmarkWriterAppend(b *testing.B) {
	b.ReportAllocs()

	const size = 1 << 10
	payload := bytes.Repeat([]byte("b"), size)
	b.SetBytes(size * 2)

	for i := 0; i < b.N; i++ {
		dst := NewWriter()
		src := NewWriter()

		if err := dst.WriteBinary(payload); err != nil {
			b.Fatal(err)
		}
		if err := src.WriteBinary(payload); err != nil {
			b.Fatal(err)
		}
		if err := dst.Append(src); err != nil {
			b.Fatal(err)
		}

		dst.Release()
	}
}

func BenchmarkWriterFlushTo(b *testing.B) {
	b.ReportAllocs()

	payload := bytes.Repeat([]byte("c"), 4<<10)
	b.SetBytes(int64(len(payload)))

	for i := 0; i < b.N; i++ {
		w := NewWriter()
		if err := w.WriteBinary(payload); err != nil {
			b.Fatal(err)
		}
		if err := w.FlushTo(io.Discard); err != nil {
			b.Fatal(err)
		}
		w.Release()
	}
}
