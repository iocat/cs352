package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"strconv"

	"github.com/iocat/rutgers-cs352/pa2/filereceiver"
	"github.com/iocat/rutgers-cs352/pa2/log"
	"github.com/iocat/rutgers-cs352/pa2/protocol"
)

var port = flag.Int("port", 9000, "A port number to reply to the sender")
var drop = flag.Int("drop", 0, "The packet dropping chance of the file receiver")
var out = flag.String("out", "./downloads", "The output folder for receiving files")

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

func main() {
	flag.Parse()
	// Set up a broadcast address
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", protocol.BroadcastPort))
	if err != nil {
		log.Warning.Fatalf("resolve address: %s", err)
	}
	// Set up a listening socket
	udpConn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Warning.Fatalf("connect through udp: %s", err)
	}
	var fr = filereceiver.New(*out, udpConn, *drop, *port)
	fr.ReceiveFiles()
}
