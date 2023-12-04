package ellsa

import "time"

type microsecs = int64
type timeTaskStatus = byte
type ioEventType = int16
type callback = func()

const (
	timerTaskDone = 1 << iota
	timerTaskDeleted
)

const (
	ioWriteEvent ioEventType = 1 << iota
	ioReadEvent
)

type timerEvent struct {
	scheduleAt microsecs
	callback   callback
}

type ioEvent struct {
	fd                          int
	eventType                   ioEventType
	readCallback, writeCallback callback
}

// AddTimerEvent adds a new timer event in the event loop
func (el *EventLoop) AddTimerEvent(scheduleAt time.Time, callback callback) bool {
	if scheduleAt.Before(time.Now()) { // schedule time is already gone
		return false
	}
	te := new(timerEvent)
	te.scheduleAt = scheduleAt.UnixMicro()
	te.callback = callback
	el.timeEvents.PushBack(te)
	return true
}

// CreateIOEvent adds a new io event in the event loop
func (el *EventLoop) AddIOEvent(fd int, eventType ioEventType, callback callback) error {
	err := el.ioEventNotifier.add(fd, eventType)
	if err != nil {
		return err
	}
	cbs := el.ioCallbacks[fd]
	if eventType == ioReadEvent {
		cbs.readCallback = callback
	} else if eventType == ioWriteEvent {
		cbs.writeCallback = callback
	}
	el.ioCallbacks[fd] = cbs
	return nil
}
