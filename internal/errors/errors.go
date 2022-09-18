package errors

import "fmt"

func New(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

func AsError(err interface{}) error {
	switch err.(type) {
	case error:
		return err.(error)
	default:
		return fmt.Errorf("%v", err)
	}
}
