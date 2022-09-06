package reader

import (
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
