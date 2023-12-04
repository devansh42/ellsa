package ellsa

import (
	"container/list"
	"math"
	"time"
)

type ioCallbacks struct {
	readCallback, writeCallback callback
}
type EventLoop struct {
	timeEvents      *list.List
	ioEventNotifier notifier
	ioCallbacks     map[int]ioCallbacks
}

func executeTimerEvent(event *timerEvent) (timeoutElapased microsecs) {
	now := time.Now().UnixMicro()
	timeoutElapased = event.scheduleAt - now
	if timeoutElapased < 0 { // schedule time passed
		event.callback()
	}
	return
}

func (el *EventLoop) executeTimerEvents() (leastTimeout microsecs) {
	ele := el.timeEvents.Front()
	leastTimeout = math.MaxInt64
	for ele != nil {
		event := ele.Value.(*timerEvent)
		timeoutElapased := executeTimerEvent(event)
		if timeoutElapased < 0 {
			nextEle := ele.Next()
			el.timeEvents.Remove(ele) // event executed, so removing from the list
			ele = nextEle
			continue
		}
		if timeoutElapased < leastTimeout {
			leastTimeout = timeoutElapased
		}
	}
	return
}

func (el *EventLoop) executeIOevents(pollTimeout microsecs) error {
	firedEvents, err := el.ioEventNotifier.poll(time.Duration(pollTimeout) * time.Microsecond)
	if err != nil {
		return err
	}
	for i := range firedEvents {
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
	if event.eventType&ioReadEvent == ioReadEvent { // we are prefering read callback before write callback
		event.readCallback()
	}
	if event.eventType&ioWriteEvent == ioWriteEvent {
		event.writeCallback()
	}
}

func (el *EventLoop) Loop() {

}
