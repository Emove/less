package errors

import "fmt"

func New(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
