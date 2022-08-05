package atomic

import (
	"sync"
	"testing"
)

func TestAtomicInt64_Dec(t *testing.T) {
	val := AtomicInt64(100)
	wg := sync.WaitGroup{}

	cnt := 100
	want := val.Value() - int64(cnt)
	wg.Add(cnt)
	for i := 0; i < cnt; i++ {
		go func() {
			val.Dec()
			wg.Done()
		}()
	}
	wg.Wait()
	if val.Value() != want {
		t.Fatal()
	}
}

func TestAtomicInt64_Inc(t *testing.T) {
	val := AtomicInt64(0)
	wg := sync.WaitGroup{}

	cnt := 100
	want := int64(cnt)
	wg.Add(cnt)
	for i := 0; i < cnt; i++ {
		go func() {
			val.Inc()
			wg.Done()
		}()
	}
	wg.Wait()
	if val.Value() != want {
		t.Fatal()
	}
}
