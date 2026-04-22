package recovery

import (
	"fmt"
	"runtime/debug"
)

func Do(fn func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v\n stack: %s", p, string(debug.Stack()))
		}
	}()

	return fn()
}

func Recover(fn func(err error)) {
	if p := recover(); p != nil {
		err := fmt.Errorf("panic error: %v\n stack: %s", p, string(debug.Stack()))
		fn(err)
	}
}
