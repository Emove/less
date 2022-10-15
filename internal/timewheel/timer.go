package timewheel

import (
	"sync"
	"time"
)

// 定时器接口
type timer interface {
	// 一次性定时器
	AfterFunc(expire time.Duration, callback func()) TimeNoder

	// 周期性定时器
	ScheduleFunc(expire time.Duration, callback func()) TimeNoder

	// 运行
	Run()

	// 停止所有定时器
	Stop()
}

// 停止单个定时器
type TimeNoder interface {
	Stop()
}

// 定时器构造函数
//func NewTimer() Timer {
//	return newTimeWheel()
//}

var Timer timer
var once = &sync.Once{}

func init() {
	once.Do(func() {
		Timer = newTimeWheel()
		go Timer.Run()
	})
}
