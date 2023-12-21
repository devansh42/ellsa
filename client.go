package ellsa

import (
	"log"
	"syscall"
)

const MAX_WRITE_PAYLOAD_SIZE = 1024 * 32   // 32 KBs
const DEFAULT_READ_BUFFER_SIZE = 1024 * 16 // 16 KBs

type client struct {
	host [4]byte
	port int
	fd   int
	// we are using a shared read buffer across all clients
	// since we are sequentially iterating over each other
	// we would have a buffer of 16kb across all clients
	readBuf []byte

	// we are gonna use a write buffer on per client basis
	// this buffer is gonna be dynamic in nature
	// we don't limit it's size
	writeBuf []byte
	// writeBufFilledUpto saves the starting index for write buffer

	writeBufFilledUpto int

	severOp serverOperator

	writeEventHandlerInstalled bool
}

func newClient(fd int, socketAddr *syscall.SockaddrInet4, servOp serverOperator, readBuf []byte) *client {
	return &client{host: socketAddr.Addr,
		port:     socketAddr.Port,
		fd:       fd,
		readBuf:  readBuf,
		writeBuf: make([]byte, 0),
		severOp:  servOp,
	}
}

func (cli *client) readCallback() {
	readBytes, err := syscall.Read(cli.fd, cli.readBuf)
	if err != nil {
		log.Print("error occured while reading from client: ", err)
		return
	}

	// we need this check as we are buffering our responses instead of sending them immediately
	if readBytes+cli.writeBufFilledUpto < len(cli.writeBuf) {
		copy(cli.writeBuf[cli.writeBufFilledUpto:], cli.readBuf[:readBytes])
	} else {
		remainingBytes := readBytes - (len(cli.writeBuf) - cli.writeBufFilledUpto)
		copy(cli.writeBuf[cli.writeBufFilledUpto:], cli.readBuf[:readBytes-remainingBytes])

		// increasing buffer size, we ain't limiting this forsake of brevity
		cli.writeBuf = append(cli.writeBuf, cli.readBuf[readBytes-remainingBytes:readBytes]...)
	}
	cli.writeBufFilledUpto += readBytes
	log.Print("writeUpto : ", cli.writeBufFilledUpto)
	log.Print("read bytes from client: ", readBytes)
	cli.severOp.addPendingWriteClient(cli)
}

func (cli *client) writeCallback() {
	var writtenBytes int
	var err error
	log.Print("insite client write callback")
	// we are fragmenting writes so that other clients get enough
	// chance to execute their requests
	if cli.writeBufFilledUpto > MAX_WRITE_PAYLOAD_SIZE {
		writtenBytes, err = syscall.Write(cli.fd, cli.writeBuf[:MAX_WRITE_PAYLOAD_SIZE])
	} else {
		writtenBytes, err = syscall.Write(cli.fd, cli.writeBuf[:cli.writeBufFilledUpto])
	}
	if err != nil {
		log.Print("error occured while reading from client: ", err)
		return
	}

	cli.writeBufFilledUpto -= writtenBytes
	if cli.writeBufFilledUpto == 0 {
		// remove from pending writes queue
		cli.severOp.removePendingWriteClient(cli.fd)
	} else {
		cli.severOp.addPendingWriteClient(cli)
	}
	log.Print("writet client: ", writtenBytes)
	log.Print("writeUpto : ", cli.writeBufFilledUpto)
}
