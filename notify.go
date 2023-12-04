package ellsa

import (
	"errors"
	"log"
	"syscall"
	"time"
)

var (
	noApplicableFilter = errors.New("no applicable filters")
	syscallKevents     = syscall.Kevent
)

type Notifier interface {
	Add(fd int, eventType IOEventType) error
	Poll(timeout time.Duration) (firedEvents []firedEvent, err error)
	Remove(fd int, eventType IOEventType) error
}

type firedEvent struct {
	fd        int
	eventType IOEventType
}

type kqueueNotifier struct {
	kqId             int
	kevents          []syscall.Kevent_t
	firedEvents      []firedEvent
	maxFd            int
	minimumQueueSize int
}

func NewKqueue(minimumQueueSize int) (Notifier, error) {
	var kn kqueueNotifier

	fd, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}
	kn.kqId = fd
	kn.minimumQueueSize = minimumQueueSize
	return &kn, nil
}

// incEventSlice dynamically increases events array size
// we are multiplying by 2 as we are polling for write and read events
func (kq *kqueueNotifier) incEventSlice(fd int) {
	if kq.maxFd < fd && len(kq.kevents) < 2*(kq.minimumQueueSize+fd) {
		delta := 2*(fd+kq.minimumQueueSize) - len(kq.kevents)
		kq.kevents = append(kq.kevents, make([]syscall.Kevent_t, delta)...)
	}

	if kq.maxFd < fd && len(kq.firedEvents) < kq.minimumQueueSize+fd {
		delta := fd + kq.minimumQueueSize - len(kq.firedEvents)
		kq.firedEvents = append(kq.firedEvents, make([]firedEvent, delta)...)
	}
	log.Print("current events size: ", len(kq.kevents))
	log.Print("current firesevents size: ", len(kq.firedEvents))
}

// setMaxFd sets max fd so far
// it should be called after incEventSlice so that we can properly increase eventSlice size
func (kq *kqueueNotifier) setMaxFd(fd int) {
	if kq.maxFd < fd {
		kq.maxFd = fd
		log.Print("current max fd: ", fd)
	}
}

func (kq *kqueueNotifier) change(fd int, flag int, eventType IOEventType) error {
	var modes []int
	if eventType&IOWriteEvent == IOWriteEvent {
		modes = append(modes, syscall.EVFILT_WRITE)
	} else if eventType&IOReadEvent == IOReadEvent {
		modes = append(modes, syscall.EVFILT_READ)
	} else {
		return noApplicableFilter
	}

	var kevents = make([]syscall.Kevent_t, len(modes))
	for i := range modes {
		var kevent syscall.Kevent_t
		syscall.SetKevent(&kevent, fd, modes[i], flag) // populating mode and flags
		kevents[i] = kevent
	}
	if len(modes) > 0 {
		_, err := syscallKevents(kq.kqId, kevents, nil, nil)
		if err != nil {
			return err
		}
		kq.incEventSlice(fd)
		kq.setMaxFd(fd)
	}
	return nil
}

func (kq *kqueueNotifier) Poll(timeout time.Duration) (events []firedEvent, err error) {
	var timespec *syscall.Timespec
	if timeout > 0 {
		ts := syscall.NsecToTimespec(timeout.Nanoseconds())
		timespec = &ts
	}
	notifications, err := syscallKevents(kq.kqId, nil, kq.kevents, timespec)
	if err != nil {
		return nil, err
	}
	var masks = make(map[uint64]IOEventType, notifications)
	for i := 0; i < notifications; i++ {
		// log.Print("filter value: ", kq.kevents[i].Filter)
		if kq.kevents[i].Filter == syscall.EVFILT_WRITE {
			masks[kq.kevents[i].Ident] |= IOWriteEvent
		} else if kq.kevents[i].Filter == syscall.EVFILT_READ {
			masks[kq.kevents[i].Ident] |= IOReadEvent
		}
	}
	// log.Printf("polling firedEvents: %+v", masks)
	eventCount := 0
	for fd := range masks {
		kq.firedEvents[eventCount].eventType = masks[fd]
		kq.firedEvents[eventCount].fd = int(fd)
		eventCount++
	}
	return kq.firedEvents[:eventCount], nil
}

func (kq *kqueueNotifier) Remove(fd int, eventType IOEventType) error {
	return kq.change(fd, syscall.EV_DELETE, eventType)
}

func (kq *kqueueNotifier) Add(fd int, eventType IOEventType) error {
	return kq.change(fd, syscall.EV_ADD, eventType)
}
