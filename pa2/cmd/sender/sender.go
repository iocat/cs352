package main

import (
	"flag"
	"net"
	"os"

	"github.com/iocat/rutgers-cs352/pa2/filesender"
	"github.com/iocat/rutgers-cs352/pa2/log"
)

var udpAddr = flag.String("addr", "255.255.255.255:8000",
	"the address:port the file sender broadcast the message to")

func main() {
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

	broadcastAddr, err := net.ResolveUDPAddr("udp", *udpAddr)
	if err != nil {
		log.Warning.Fatalf("resolve udp address: %s", err)
	}

	udpConn, err := net.DialUDP("udp", nil, broadcastAddr)
	if err != nil {
		log.Warning.Fatalf("connect to the address: %s", err)
	}
	// Start the sender process
	fs := filesender.New(udpConn, files)
	fs.Run()
}
