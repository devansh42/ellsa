package main

import (
	"log"

	"github.com/devansh42/ellsa"
)

var localhostAddr = [4]byte{127, 0, 0, 1}

func main() {
	el := ellsa.NewEventLoop()
	ioEventNotifier, err := ellsa.NewKqueue(10)
	if err != nil {
		log.Fatal("couldn't initialise kqueue notifier: ", err)
	}
	el.SetNotifier(ioEventNotifier)
	serv := ellsa.NewServer(9999, localhostAddr, el)
	serv.StartServer()
	el.Loop()
}
