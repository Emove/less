package transport

// Options represents custom transport Config Options
type Options interface{}

// Option represents config func
// for example:
// 		func WithDeadline(t time.Time) Option {
//			return func (ops Options) {
//				ops.(*TCPOptions).deadline = t
//			}
//		}
type Option func(ops Options)
