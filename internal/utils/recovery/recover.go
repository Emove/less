package recovery

import (
	"github.com/emove/less/internal/errors"
	"runtime/debug"
)

func Do(fn func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = errors.New("panic: %v\n stack: %s", p, string(debug.Stack()))
		}
	}()

	return fn()
}

func Recover(fn func(err error)) {
	if p := recover(); p != nil {
		err := errors.New("panic error: %v\n stack: %s", p, string(debug.Stack()))
		fn(err)
	}
}
