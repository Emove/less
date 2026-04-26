package framebuf

import "errors"

var ErrReleased = errors.New("framebuf: buffer has been released")
