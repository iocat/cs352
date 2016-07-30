package filereceiver

import (
	"fmt"
	"math/rand"
	"net"
	"os"

	"github.com/iocat/rutgers-cs352/pa2/log"
	"github.com/iocat/rutgers-cs352/pa2/protocol"
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram"
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram/header"
	"github.com/iocat/rutgers-cs352/pa2/protocol/sender"
	"github.com/iocat/rutgers-cs352/pa2/protocol/window"
)

// FileReceiver represents a receiver that receives file packets from
// UDP Connection
type FileReceiver struct {
	senderAddr net.Addr

	droppingChance int

	// current header is the current header of the latest packet
	currentHeader header.Header

	// The window this FileReceiver uses to store data
	window *window.Window

	*net.UDPConn
}

// New creates a new FileReceiver object
func New(conn *net.UDPConn, droppingChance int) *FileReceiver {
	if droppingChance < 0 || droppingChance > 100 {
		log.Warning.Fatal("dropping chance out of range: should be between 0 and 100")
	}
	return &FileReceiver{
		UDPConn: conn,
		window:  window.New(protocol.WindowSize),
	}
}

// receiveData receives data buffer and send it to the newData channel
func (fileReceiver *FileReceiver) receiveData(
	newData chan<- []byte,
	stopSignal <-chan struct{}) {
	var (
		data []byte
	)
loop:
	for {
		select {
		case <-stopSignal:
			break loop
		default:
			data = make([]byte, protocol.SegmentSize)
			length, addr, err := fileReceiver.ReadFrom(data)
			if err != nil {
				log.Warning.Printf("receive data error: %s", err)
			}
			if fileReceiver.senderAddr == nil {
				log.Info.Println("new sender dectected: set this sender as an official sender")
				fileReceiver.senderAddr = addr
			}
			// Check sender address
			if addr.String() != fileReceiver.senderAddr.String() {
				log.Warning.Println("receive broadcast packet from unknown sender host")
			}
			// check the packet length
			if length < protocol.HeaderSize {
				log.Debug.Printf("packet size is not correct: expected >%d bytes, received %d bytes",
					protocol.HeaderSize, length)
			}
			// pass it up
			newData <- data[:length]
		}
	}
}

func validateDir(outputDir *os.File) error {
	var (
		dir os.FileInfo
		err error
	)
	if dir, err = outputDir.Stat(); err != nil {
		return err
	}
	if !dir.IsDir() {
		return fmt.Errorf("%s is not a directory", outputDir.Name())
	}
	return nil
}

func toDrop(droppingChance int) bool {
	return rand.Intn(100) < droppingChance
}

// ReceiveFiles starts receiving files
// outputDir is the directory to write the downloaded files to
// TODO: implement reordering of packets
func (fileReceiver *FileReceiver) ReceiveFiles(outputDir *os.File) {
	/*if err := validateDir(outputDir); err != nil {
		log.Warning.Fatalf("validate output directory: %s", err)
	}*/
	var (
		newData       = make(chan []byte)
		stopReceiving = make(chan struct{})

		reconstructData = make(chan []byte)
		reconstructDone = make(chan struct{})
		currentFile     *os.File
		err             error
	)
	go fileReceiver.receiveData(newData, stopReceiving)
loop:
	for data := range newData {
		segment := datagram.NewFromUDPPayload(data)
		// try to drop the packet
		if toDrop(fileReceiver.droppingChance) {
			log.Warning.Printf("pseudo packet drop: drop one with header: %#v",
				segment.Header)
			continue
		}
		switch {
		case segment.IsFILE():
			if currentFile == nil {
				currentFile, err = os.Create(string(segment.Payload))
				if err != nil {
					log.Warning.Fatalf("Unable to create file: %s", err)
				}
				go log.Info.Println("New FILE request: spawned a file reconstructing thread")
				go reconstructFile(currentFile, reconstructData, reconstructDone)
			} else {
				// File duplication
				if currentFile.Name() == string(segment.Payload) {
					continue
				}
			}
		case segment.IsEXIT():
			if currentFile != nil {
				// Send an eof signal
				sendEOF(reconstructData, reconstructDone)
				currentFile = nil
				break loop
			}
		case segment.IsEOF():
			if currentFile != nil {
				sendEOF(reconstructData, reconstructDone)
				currentFile = nil
			}
		// Normal file packet
		default:
			if currentFile == nil {
				log.Warning.Println("Received data packet but no file is set up.")
			} else {
				reconstructData <- segment.Payload
			}
			// Send an ACK back
			sender.New(fileReceiver.UDPConn,
				datagram.New(
					header.ACK|segment.Header.Flag,
					segment.Header.Sequence,
					nil)).SendTo(fileReceiver.senderAddr)
		}
	}
}

func sendEOF(reconstructData chan<- []byte, reconstructDone <-chan struct{}) {
	// Close the file and wait til the file is closed
	reconstructData <- nil
	<-reconstructDone
}
