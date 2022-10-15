package recovery

import "testing"

func TestDo(t *testing.T) {
	t.Logf("%s", Do(func() error {
		panic("fake panic")
	}).Error())
}

func TestRecover(t *testing.T) {
	defer Recover(func(err error) {
		t.Logf(err.Error())
	})

	panic("fake panic")
}
