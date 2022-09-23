package _go

import (
	"runtime/debug"
	"time"

	"github.com/emove/less/log"
	"github.com/panjf2000/ants/v2"
)

var (
	// DefaultAntsPoolSize sets up the capacity of worker pool, 256 * 1024.
	DefaultAntsPoolSize = 1 << 18
)

const (
	// ExpiryDuration is the interval time to clean up those expired workers.
	ExpiryDuration = 10 * time.Second

	// Nonblocking decides what to do when submitting a new task to a full worker pool: waiting for a available worker
	// or returning nil directly.
	Nonblocking = true
)

type logger struct {
}

func (*logger) Printf(format string, a ...interface{}) {
	log.Errorf(format, a...)
}

func init() {
	// It releases the default pool from ants.
	ants.Release()
}

// Pool is the alias of ants.Pool.
type Pool = ants.Pool

var global *Pool

// Init instantiates a non-blocking *WorkerPool with the capacity of DefaultAntsPoolSize.
func Init() {
	if global != nil {
		global.Release()
	}
	options := ants.Options{
		ExpiryDuration: ExpiryDuration,
		Nonblocking:    Nonblocking,
		PanicHandler: func(err interface{}) {
			log.Errorf("panic on worker: %v,\n %s", err, string(debug.Stack()))
		},
		Logger: &logger{},
	}
	global, _ = ants.NewPool(DefaultAntsPoolSize, ants.WithOptions(options))
}

func Submit(task func()) {
	if global != nil {
		err := global.Submit(task)
		if err == nil {
			return
		}
		log.Warnw("goroutine pool err", err)
	}
	go task()
}

func Release() {
	if global != nil {
		global.Reboot()
	}
}
