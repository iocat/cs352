package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/iocat/rutgers-cs352/pa2/filesender"
	"github.com/iocat/rutgers-cs352/pa2/log"
	"github.com/iocat/rutgers-cs352/pa2/protocol"
)

var window = flag.Int("wind", protocol.WindowSize, "The size of the receiver window")

var broadcastAddr = flag.String("baddr", "255.255.255.255",
	"The broadcast address this sender is broadcasting to")

var listeningPort = flag.Int("port", 9000,
	fmt.Sprintf("The port number this sender receives ACKs, must not be the broadcast port% d",
		protocol.BroadcastPort))

var drop = flag.Int("drop", 0, "The probability to drop one receiving packet")

func main() {
	// Create a listening socket
	listenSocket, err := getListenSocket(*listeningPort)
	if err != nil {
		log.Warning.Fatalf(err.Error())
	}
	// Create a broadcast socket
	broadcastSocket, err := getBroadcastSocket(*broadcastAddr)
	if err != nil {
		log.Warning.Fatalf(err.Error())
	}

	flag.Parse()
	var (
		files []*os.File
	)
	if len(flag.Args()) == 0 {
		log.Warning.Fatalln("not enough argument, please provide some file names that are being sent")
	}

	for i, f := range flag.Args() {
		var (
			open *os.File
			err  error
		)
		if open, err = os.Open(f); err != nil {
			log.Warning.Fatalf("cannot open %d-th file: %s", i, err)
		}
		files = append(files, open)
	}

	// Start the sender process
	fs := filesender.NewWithWindowSize(*window, broadcastSocket, listenSocket, files)
	fs.DroppingChance = *drop
	fs.Run()
}

func getListenSocket(listenPort int) (*net.UDPConn, error) {
	listenAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", listenPort))
	if err != nil {
		return nil, fmt.Errorf("resolve udp listening address: %s", err)
	}
	listenSocket, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("bind to the listening address: %s", err)
	}
	return listenSocket, nil
}

func getBroadcastSocket(broadcastIP string) (*net.UDPConn, error) {
	broadcastAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", broadcastIP, protocol.BroadcastPort))
	if err != nil {
		return nil, fmt.Errorf("resolve udp broadcast address: %s", err)
	}

	udpConn, err := net.DialUDP("udp", nil, broadcastAddr)
	if err != nil {
		return nil, fmt.Errorf("connect to the address: %s", err)
	}
	return udpConn, nil
}
