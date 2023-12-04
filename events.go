package ellsa

import "time"

type microsecs = int64
type IOEventType = int16
type callback = func()

const (
	IOWriteEvent IOEventType = 1 << iota
	IOReadEvent
)

type timerEvent struct {
	scheduleAt microsecs
	callback   callback
}

type ioEvent struct {
	fd                          int
	eventType                   IOEventType
	readCallback, writeCallback callback
}

// AddTimerEvent adds a new timer event in the event loop
func (el *eventLoop) AddTimerEvent(scheduleAt time.Time, callback callback) bool {
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
func (el *eventLoop) AddIOEvent(fd int, eventType IOEventType, callback callback) error {
	err := el.ioEventNotifier.Add(fd, eventType)
	if err != nil {
		return err
	}
	cbs := el.ioCallbacks[fd]
	if eventType == IOReadEvent {
		cbs.readCallback = callback
	} else if eventType == IOWriteEvent {
		cbs.writeCallback = callback
	}
	el.ioCallbacks[fd] = cbs
	return nil
}
