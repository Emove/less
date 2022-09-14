package reader

import (
	"bytes"
	"testing"
)

func TestNewLimitReader(t *testing.T) {
	do(func(pair *connPair) {
		reader := NewLimitReader(NewBufferReader(pair.server), 6)
		t.Logf("%#v", reader)
	})
}

func Test_limitReader_Next(t *testing.T) {
	do(func(pair *connPair) {
		reader := NewLimitReader(NewBufferReader(pair.server), 6).(*limitReader)
		_, _ = pair.client.Write([]byte("hello server"))
		next, err := reader.Next(6)
		if err != nil {
			t.Fatalf("read next 6 bytes error: %s", err.Error())
		}
		t.Log(string(next))
		t.Logf("limitReader remain: %d", reader.remain)

		next, err = reader.Next(1)
		if err != nil {
			t.Logf("read next 1 bytes error: %s", err.Error())
		} else {
			t.Fatalf("read more bytes: %s", string(next))
		}
	})
}

func Test_limitReader_Peek(t *testing.T) {
	do(func(pair *connPair) {
		reader := NewLimitReader(NewBufferReader(pair.server), 6).(*limitReader)
		_, _ = pair.client.Write([]byte("hello server"))
		next, err := reader.Peek(6)
		if err != nil {
			t.Fatalf("read next 6 bytes error: %s", err.Error())
		}
		t.Log(string(next))
		t.Logf("limitReader remain: %d", reader.remain)

		_, _ = reader.Next(6)
		next, err = reader.Peek(1)
		if err != nil {
			t.Logf("read next 1 bytes error: %s", err.Error())
		} else {
			t.Fatalf("read more bytes: %s", string(next))
		}
	})
}

func Test_limitReader_Release(t *testing.T) {
	do(func(pair *connPair) {
		reader := NewLimitReader(NewBufferReader(pair.server), 6).(*limitReader)
		_, _ = pair.client.Write([]byte("hello server"))
		peek, err := reader.Peek(5)
		if err != nil {
			t.Fatalf("read next 6 bytes error: %s", err.Error())
		}
		t.Log(string(peek))
		t.Logf("limitReader remain: %d", reader.remain)

		reader.Release()
		t.Logf("limitReader remain: %d", reader.remain)
	})
}

func Test_limitReader_Skip(t *testing.T) {
	do(func(pair *connPair) {
		reader := NewLimitReader(NewBufferReader(pair.server), 6).(*limitReader)
		_, _ = pair.client.Write([]byte("hello server"))
		err := reader.Skip(6)
		if err != nil {
			t.Fatalf("read next 6 bytes error: %s", err.Error())
		}
		t.Logf("limitReader remain: %d", reader.remain)

		next, err := reader.Peek(1)
		if err != nil {
			t.Logf("read next 1 bytes error: %s", err.Error())
		} else {
			t.Fatalf("read more bytes: %s", string(next))
		}
	})
}

func Test_limitReader_Length(t *testing.T) {
	buff := &bytes.Buffer{}
	buff.WriteString("hello world")

	reader := NewLimitReader(NewBufferReader(buff), 11)
	if reader.Length() != 11 {
		t.Fatalf("want: %d, got: %d", 11, reader.Length())
	}
}

func Test_limitReader_Read(t *testing.T) {
	buff := &bytes.Buffer{}
	buff.WriteString("hello world")

	reader := NewLimitReader(NewBufferReader(buff), 11)

	h := make([]byte, 5, 5)
	read, err := reader.Read(h)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if read != 5 {
		t.Fatalf("want n: %d, got n: %d", 5, read)
	}

	if string(h) != "hello" {
		t.Fatalf("want buff: %s, got buff: %s", "hello", string(h))
	}

	// readIndex < writerIndex, but enough
	_, _ = reader.Peek(1)
	b := make([]byte, 1, 1)
	read, err = reader.Read(b)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if read != 1 {
		t.Fatalf("want n: %d, got n: %d", 1, read)
	}

	if string(b) != " " {
		t.Fatalf("want buff: %s, got buff: %s", " ", string(b))
	}

	// readIndex < writeIndex, but inside buff not enough
	_, _ = reader.Peek(3)
	s := make([]byte, 5, 5)
	read, err = reader.Read(s)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if read != 5 {
		t.Fatalf("want n: %d, got n: %d", 5, read)
	}

	if string(s) != "world" {
		t.Fatalf("want buff: %s, got buff: %s", "world", string(s))
	}

	b = make([]byte, 1, 1)
	read, err = reader.Read(b)
	if err != nil {
		t.Logf("got: %v", err)
	}
}
