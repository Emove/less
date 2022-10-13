package channel

import "sync"

const (
	ReadEvent = iota
	WriteEvent
)

type WaitGroup struct {
	readTask  *sync.WaitGroup
	writeTask *sync.WaitGroup
	allTask   *sync.WaitGroup
}

func NewWaitGroup() *WaitGroup {
	return &WaitGroup{
		readTask:  &sync.WaitGroup{},
		writeTask: &sync.WaitGroup{},
		allTask:   &sync.WaitGroup{},
	}
}

func (wg *WaitGroup) Add(event int) {
	switch event {
	case ReadEvent:
		wg.readTask.Add(1)
	case WriteEvent:
		wg.writeTask.Add(1)
	default:
		return
	}
	wg.allTask.Add(1)
}

func (wg *WaitGroup) Done(event int) {
	switch event {
	case ReadEvent:
		wg.readTask.Done()
	case WriteEvent:
		wg.writeTask.Done()
	default:
		return
	}
	wg.allTask.Done()
}

func (wg *WaitGroup) WaitReadTask() {
	wg.readTask.Wait()
}

func (wg *WaitGroup) WaitWriteTask() {
	wg.writeTask.Wait()
}

func (wg *WaitGroup) Wait() {
	wg.allTask.Wait()
}
