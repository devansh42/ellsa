package ellsa

import (
	"bytes"
	"io"
	"syscall"
	"testing"
)

func TestLoop(t *testing.T) {

	notifier, err := NewKqueue(10)
	if err != nil {
		t.Error("error while initialising kqueue: ", err)
		return
	}
	loop := NewEventLoop()
	loop.SetNotifier(notifier)

	setupServer(t, loop)
	loop.Loop()
}

func makeNonBlocking(fd int) error {
	return syscall.FcntlFlock(uintptr(fd), syscall.F_SETFL, &syscall.Flock_t{Type: syscall.O_NONBLOCK})
}

func setupServer(t *testing.T, loop EventLoop) {

	serverFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Error("err while creating socket: ", err)
		return
	}
	err = makeNonBlocking(serverFd)
	if err != nil {
		t.Error("err while making server socket non blocking: ", err)
		return
	}
	addr := [4]byte{127, 0, 0, 1}
	sockAddr := syscall.SockaddrInet4{Port: 9999, Addr: addr}
	err = syscall.Bind(serverFd, &sockAddr)
	if err != nil {
		t.Error("err while binding socket: ", err)
		return
	}
	err = syscall.Listen(serverFd, 128)
	if err != nil {
		t.Error("err while listing the socket: ", err)
		return
	}
	// Adding this listener to event queue
	loop.AddIOEvent(serverFd, IOReadEvent, accpectorFn(t, loop, serverFd))
	t.Log("Server started Successfully")
}

func accpectorFn(t *testing.T, loop EventLoop, fd int) callback {

	return func() {
		t.Log("got a new client")
		clientFd, _, err := syscall.Accept(fd)
		if err != nil {
			t.Error("err while accepting incoming connection: ", err)
			return
		}
		err = makeNonBlocking(clientFd)
		if err != nil {
			t.Error("err while changing incoming socket mode to non blocking: ", err)
			return
		}
		t.Log("accepted client fd: ", clientFd)
		var buf bytes.Buffer
		loop.AddIOEvent(clientFd, IOReadEvent, readerFn(t, &buf, clientFd))
		loop.AddIOEvent(clientFd, IOWriteEvent, writerFn(t, &buf, clientFd))
	}
}

func writerFn(t *testing.T, rd io.Reader, fd int) callback {
	var buf = make([]byte, 32)
	return func() {
		readBytes, err := io.ReadFull(rd, buf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			t.Error("err while reading from pipe: ", err)
			return
		}
		if readBytes > 0 {
			_, err = syscall.Write(fd, buf[:readBytes])
			if err != nil {
				t.Error("error occured while writing to socket: ", err)
			}
		}
	}
}

func readerFn(t *testing.T, wr io.Writer, fd int) callback {
	var buf = make([]byte, 32)
	return func() {
		readBytes, err := syscall.Read(fd, buf)
		if err != nil {
			t.Error("err while reading to socket: ", err)
			return
		}
		t.Log("got read payload: ", buf[:readBytes])
		_, err = wr.Write(buf[:readBytes])
		if err != nil {
			t.Error("err while writting to in-memory socket: ", err)
			return
		}

	}
}
