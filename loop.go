package ellsa

import (
	"container/list"
	"log"
	"math"
	"time"
)

type ioCallbacks struct {
	readCallback, writeCallback callback
}
type eventLoop struct {
	timeEvents      *list.List
	ioEventNotifier Notifier
	ioCallbacks     map[int]ioCallbacks
	// setting stopLoop would terminate the eventLoop
	stopLoop bool
}

type EventLoop interface {
	Loop()
	Stop()
	SetNotifier(Notifier)
	AddTimerEvent(scheduleAt time.Time, callback callback) bool
	AddIOEvent(fd int, eventType IOEventType, callback callback) error
}

func NewEventLoop() EventLoop {
	var loop = eventLoop{
		timeEvents:  list.New(),
		ioCallbacks: make(map[int]ioCallbacks),
	}
	return &loop
}

func (el *eventLoop) SetNotifier(notifier Notifier) {
	el.ioEventNotifier = notifier
}

func (el *eventLoop) Stop() {
	el.stopLoop = true
}
func executeTimerEvent(event *timerEvent) (timeoutElapased microsecs) {
	now := time.Now().UnixMicro()
	timeoutElapased = event.scheduleAt - now
	if timeoutElapased < 0 { // schedule time passed
		event.callback()
	}
	return
}

func (el *eventLoop) executeTimerEvents() (msLeastTimeout microsecs) {
	ele := el.timeEvents.Front()
	msLeastTimeout = math.MaxInt64
	leastTimeoutChanged := false
	for ele != nil {
		event := ele.Value.(*timerEvent)
		timeoutElapased := executeTimerEvent(event)
		if timeoutElapased < 0 {
			nextEle := ele.Next()
			el.timeEvents.Remove(ele) // event executed, so removing from the list
			ele = nextEle
			continue
		}
		if timeoutElapased < msLeastTimeout {
			msLeastTimeout = timeoutElapased
			leastTimeoutChanged = true
		}
	}
	if !leastTimeoutChanged {
		return -1
	}
	return
}

func (el *eventLoop) executeIOevents(pollTimeout microsecs) error {
	firedEvents, err := el.ioEventNotifier.Poll(time.Duration(pollTimeout) * time.Microsecond)
	if err != nil {
		log.Print("error while polling")
		return err
	}
	for i := range firedEvents {
		// log.Print("fired event fd: ", firedEvents[i].fd)
		var event = ioEvent{
			fd:            firedEvents[i].fd,
			eventType:     firedEvents[i].eventType,
			readCallback:  el.ioCallbacks[firedEvents[i].fd].readCallback,
			writeCallback: el.ioCallbacks[firedEvents[i].fd].writeCallback,
		}
		executeIOEvent(&event)
	}
	return nil
}

func executeIOEvent(event *ioEvent) {
	if event.eventType&IOReadEvent == IOReadEvent { // we are prefering read callback before write callback
		// log.Print("invoking read callback for fd: ", event.fd)
		event.readCallback()
	}
	if event.eventType&IOWriteEvent == IOWriteEvent {
		// log.Print("invoking write callback for fd: ", event.fd)
		event.writeCallback()
	}
}

func (el *eventLoop) Loop() {
	for !el.stopLoop {

		leastTimeout := el.executeTimerEvents()
		err := el.executeIOevents(leastTimeout)
		if err != nil {
			log.Fatal("err occured while executing IO events: ", err)
		}
	}
}
