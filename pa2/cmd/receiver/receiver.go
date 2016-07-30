package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"strconv"

	"github.com/iocat/rutgers-cs352/pa2/filereceiver"
	"github.com/iocat/rutgers-cs352/pa2/log"
)

func parseArgs(args []string) (port, drop int, err error) {
	if port, err = strconv.Atoi(args[1]); err != nil {
		return
	}
	if drop, err = strconv.Atoi(args[2]); err != nil {
		return
	}
	if drop < 0 || drop > 100 {
		err = errors.New("the receiver's packet dropping chance should be in the range of [0,100]")
	}
	return
}

var port = flag.Int("addr", 8000, "the port number to listen to")
var droppingChance = flag.Int("drop", 0, "the packet dropping chance of the file receiver")

func main() {
	flag.Parse()
	// Start a new receiver
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Warning.Fatalf("resolve address: %s", err)
	}
	udpConn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Warning.Fatalf("connect through udp: %s", err)
	}
	var fr = filereceiver.New(udpConn, *droppingChance)
	fr.ReceiveFiles(nil)
}
