package ellsa

import (
	"errors"
	"syscall"
	"time"
)

var (
	noApplicableFilter = errors.New("no applicable filters")
)

type notifier interface {
	add(fd int, eventType ioEventType) error
	poll(timeout time.Duration) (firedEvents []firedEvent, err error)
	remove(fd int, eventType ioEventType) error
}

type firedEvent struct {
	fd        int
	eventType ioEventType
}

type kqueueNotifier struct {
	kqId             int
	kevents          []syscall.Kevent_t
	firedEvents      []firedEvent
	maxFd            int
	minimumQueueSize int
}

func newKqueue(minimumQueueSize int) (notifier, error) {
	var kn kqueueNotifier

	fd, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}
	kn.kqId = fd
	kn.minimumQueueSize = minimumQueueSize
	return nil, nil
}

// incEventSlice dynamically increases events array size
// we are multiplying by 2 as we are polling for write and read events
func (kq *kqueueNotifier) incEventSlice(fd int) {
	if kq.maxFd < fd && cap(kq.kevents) < 2*(kq.minimumQueueSize+fd) {
		delta := fd + kq.minimumQueueSize - cap(kq.kevents)
		delta *= 2
		kq.kevents = append(kq.kevents, make([]syscall.Kevent_t, delta)...)
	}

	if kq.maxFd < fd && cap(kq.firedEvents) < kq.minimumQueueSize+fd {
		delta := fd + kq.minimumQueueSize - cap(kq.firedEvents)
		kq.firedEvents = append(kq.firedEvents, make([]firedEvent, delta)...)
	}
}

// setMaxFd sets max fd so far
// it should be called after incEventSlice so that we can properly increase eventSlice size
func (kq *kqueueNotifier) setMaxFd(fd int) {
	if kq.maxFd < fd {
		kq.maxFd = fd
	}
}

func (kq *kqueueNotifier) change(fd int, flag int, eventType ioEventType) error {

	var modes []int
	if eventType&ioWriteEvent == ioWriteEvent {
		modes = append(modes, syscall.EVFILT_WRITE)
	} else if eventType&ioReadEvent == ioReadEvent {
		modes = append(modes, syscall.EVFILT_READ)
	} else {
		return noApplicableFilter
	}

	var kevent = new(syscall.Kevent_t)
	for i := range modes {
		syscall.SetKevent(kevent, fd, modes[i], flag) // populating mode and flags
		_, err := syscall.Kevent(kq.kqId, []syscall.Kevent_t{*kevent}, nil, nil)
		if err != nil {
			return err
		}
	}
	if len(modes) > 0 {
		kq.incEventSlice(fd)
		kq.setMaxFd(fd)
	}
	return nil
}

func (kq *kqueueNotifier) poll(timeout time.Duration) (events []firedEvent, err error) {
	timespec := syscall.NsecToTimespec(timeout.Nanoseconds())
	notifications, err := syscall.Kevent(kq.kqId, nil, kq.kevents, &timespec)
	if err != nil {
		return nil, err
	}
	var masks = make(map[uint64]int16, notifications)
	for i := 0; i < notifications; i++ {
		masks[kq.kevents[i].Ident] |= kq.kevents[i].Filter
	}
	eventCount := 0
	for fd := range masks {
		kq.firedEvents[eventCount].eventType = masks[fd]
		eventCount++
	}
	return kq.firedEvents[:eventCount], nil
}

func (kq *kqueueNotifier) remove(fd int, eventType ioEventType) error {
	return kq.change(fd, syscall.EV_DELETE, eventType)
}

func (kq *kqueueNotifier) add(fd int, eventType ioEventType) error {
	return kq.change(fd, syscall.EV_ADD, eventType)
}
