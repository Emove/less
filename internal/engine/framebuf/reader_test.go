package framebuf

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type chunkReader struct {
	chunks [][]byte
	index  int
	err    error
}

func newChunkReader(chunks ...string) *chunkReader {
	r := &chunkReader{chunks: make([][]byte, 0, len(chunks))}
	for _, chunk := range chunks {
		r.chunks = append(r.chunks, []byte(chunk))
	}
	return r
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		if r.err != nil {
			return 0, r.err
		}
		return 0, io.EOF
	}
	chunk := r.chunks[r.index]
	r.index++
	return copy(p, chunk), nil
}

func newTestReaderWithBlockSize(src io.Reader, blockSize int) *reader {
	if blockSize <= 0 {
		blockSize = 256
	}
	return &reader{src: src, buf: make([]byte, 0, blockSize)}
}

func TestReader_NextDoesNotCompactReturnedBorrowedSlice(t *testing.T) {
	firstChunk := bytes.Repeat([]byte("a"), 300)
	secondChunk := bytes.Repeat([]byte("b"), 300)
	rd := NewReader(bytes.NewBuffer(append(firstChunk, secondChunk...)))

	first, err := rd.Next(300)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if got := string(first); got != string(firstChunk) {
		t.Fatalf("first Next() = %q, want %q", got, string(firstChunk))
	}

	second, err := rd.Next(300)
	if err != nil {
		t.Fatalf("second Next() error = %v", err)
	}
	if got := string(second); got != string(secondChunk) {
		t.Fatalf("second Next() = %q, want %q", got, string(secondChunk))
	}
	if got := string(first); got != string(firstChunk) {
		t.Fatalf("borrowed first slice changed to %q after second read", got)
	}
}

func TestReader_PeekAndNextReturnContiguousBytesAcrossNodes(t *testing.T) {
	rd := newTestReaderWithBlockSize(newChunkReader("ab", "cd", "ef"), 2)

	peek, err := rd.Peek(5)
	if err != nil {
		t.Fatalf("Peek() error = %v", err)
	}
	if got := string(peek); got != "abcde" {
		t.Fatalf("Peek() = %q, want %q", got, "abcde")
	}

	next, err := rd.Next(5)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if got := string(next); got != "abcde" {
		t.Fatalf("Next() = %q, want %q", got, "abcde")
	}

	tail, err := rd.Next(1)
	if err != nil {
		t.Fatalf("tail Next() error = %v", err)
	}
	if got := string(tail); got != "f" {
		t.Fatalf("tail Next() = %q, want %q", got, "f")
	}
}

func TestReader_SliceAcrossNodesSurvivesReaderRelease(t *testing.T) {
	rd := newTestReaderWithBlockSize(newChunkReader("ab", "cd", "ef"), 2)

	frame, err := rd.Slice(6)
	if err != nil {
		t.Fatalf("Slice() error = %v", err)
	}
	defer frame.Release()

	rd.Release()

	if got := string(frame.Bytes()); got != "abcdef" {
		t.Fatalf("frame.Bytes() = %q, want %q", got, "abcdef")
	}
}

func TestReader_EOFBeforeRequestedBytesReturnsUnexpectedEOF(t *testing.T) {
	rd := NewReader(bytes.NewBufferString("abc"))

	_, err := rd.Peek(4)
	if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		t.Fatalf("Peek() error = %v, want EOF-compatible error", err)
	}
}

func TestReader_ReleaseIsIdempotentAndRejectsFurtherReads(t *testing.T) {
	rd := NewReader(bytes.NewBufferString("abc"))
	rd.Release()
	rd.Release()

	if _, err := rd.Peek(1); !errors.Is(err, ErrReleased) {
		t.Fatalf("Peek() after Release error = %v, want %v", err, ErrReleased)
	}
}
