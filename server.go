package ellsa

import (
	"log"
	"syscall"
)

type Server struct {
	port                int
	host                [4]byte
	fd                  int
	loop                EventLoop
	pendingWritesClient map[int]*client
	cliReadBuf          []byte
}

type serverOperator interface {
	removePendingWriteClient(fd int) (found bool)
	addPendingWriteClient(cli *client) (added bool)
}

func NewServer(port int, host [4]byte, loop EventLoop) *Server {
	serv := Server{
		port:                port,
		host:                host,
		pendingWritesClient: make(map[int]*client),
		loop:                loop,
		cliReadBuf:          make([]byte, DEFAULT_READ_BUFFER_SIZE),
	}
	return &serv
}

func (serv *Server) StartServer() {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		log.Fatal("error occured while starting server socket: ", err)
	}
	serv.fd = fd
	sockAddr := syscall.SockaddrInet4{Port: serv.port, Addr: serv.host}
	err = syscall.Bind(fd, &sockAddr)
	if err != nil {
		log.Fatal("couldn't bind server socket to address: ", err)
	}
	// This is a maximum limit of connection we can waitting in a queue
	err = syscall.Listen(fd, 128)
	if err != nil {
		log.Fatal("err while listening the socket: ", err)
	}
	err = makeNonBlocking(fd) // marking server socket non blocking, so accept calls won't block
	if err != nil {
		log.Fatal("err while making server listerner non blocking")
	}
	serv.loop.AddIOEvent(fd, IOReadEvent, serv.acceptConnectionCallback)
	serv.setupBeforePollCallback()
	log.Print("Ellsa Started at: ", serv.port)
}

// acceptConnectionCallback would get called incase of any new incoming connection
func (serv *Server) acceptConnectionCallback() {
	incomingConnFd, iconnAddr, err := syscall.Accept(serv.fd) // accepting the connection, one at a time
	if err != nil {
		log.Print("error while accepting incoming client: ", err)
		return
	}
	connAddr := iconnAddr.(*syscall.SockaddrInet4)
	err = makeNonBlocking(incomingConnFd)
	if err != nil {
		log.Print("couldn't make connection non blocking")
		return
	}
	cli := newClient(incomingConnFd, connAddr, serv, serv.cliReadBuf)
	// added read callback for client socket
	serv.loop.AddIOEvent(incomingConnFd, IOReadEvent, cli.readCallback)
	log.Println("New Client Accepted: ", connAddr.Port)
}

func (serv *Server) addPendingWriteClient(cli *client) (added bool) {
	_, ok := serv.pendingWritesClient[cli.fd]
	if !ok {
		serv.pendingWritesClient[cli.fd] = cli
	}
	return !ok
}

func (serv *Server) setupBeforePollCallback() {

	serv.loop.SetBeforePollCallback(func(el EventLoop) {
		log.Print("inside before poll callback")
		serv.managePendingClientWrites(el)
	})

}

func (serv *Server) removePendingWriteClient(fd int) (found bool) {
	_, ok := serv.pendingWritesClient[fd]
	if ok {
		delete(serv.pendingWritesClient, fd)
	}
	return ok
}

func makeNonBlocking(fd int) error {
	return syscall.FcntlFlock(uintptr(fd), syscall.F_SETFL, &syscall.Flock_t{Type: syscall.O_NONBLOCK})
}

// TODO : We are making one syscall for each client and these calls
// can be clubbed to one syscall only
func (serv *Server) managePendingClientWrites(el EventLoop) {
	var err error
	for fd, cli := range serv.pendingWritesClient {
		cli.writeCallback()
		// let's check if still have this cli in pending writes map
		// as writeCallback removes them from the map, they don't have pending writes
		_, havePendingWrites := serv.pendingWritesClient[fd]
		if havePendingWrites && !cli.writeEventHandlerInstalled {
			// lets install write handler
			err = el.AddIOEvent(fd, IOWriteEvent, cli.writeCallback)
			if err != nil {
				log.Print("error occured while adding write handler: ", err)
				continue
			}
			cli.writeEventHandlerInstalled = true
		} else if !havePendingWrites && cli.writeEventHandlerInstalled {
			// let's uninstall write handler as it's not required
			err = el.RemoveIOEvent(fd, IOWriteEvent)
			if err != nil {
				log.Print("error occured while removing write handler: ", err)
				continue
			}
			cli.writeEventHandlerInstalled = false
		}
	}
}
