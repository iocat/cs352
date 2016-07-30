package filereceiver

import (
	"errors"
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

	reconstructData chan []byte
	reconstructDone chan struct{}
	currentFile     *os.File
}

// New creates a new FileReceiver object
func New(conn *net.UDPConn, droppingChance int) *FileReceiver {
	if droppingChance < 0 || droppingChance > 100 {
		log.Warning.Fatal("dropping chance out of range: should be between 0 and 100")
	}
	return &FileReceiver{
		UDPConn:         conn,
		window:          window.New(protocol.WindowSize),
		reconstructData: make(chan []byte),
		reconstructDone: make(chan struct{}),
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
				log.Warning.Printf("receive broadcast packet from unknown sender host, got %s, expected %s",
					addr.String(), fileReceiver.senderAddr.String())
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
	var (
		newData       = make(chan []byte)
		stopReceiving = make(chan struct{})

		expectedHeader = header.Header{
			Flag:     header.RED,
			Sequence: 0,
		}
	)
	go fileReceiver.receiveData(newData, stopReceiving)
loop:
	for data := range newData {
		segment := newReceiverSegment(datagram.NewFromUDPPayload(data))
		// try to drop the packet
		if toDrop(fileReceiver.droppingChance) {
			log.Warning.Printf("pseudo packet drop: drop one with header: %#v",
				segment.Header)
			continue
		} else {
			// Accepted: Send an ACK back
			go sender.New(fileReceiver.UDPConn,
				datagram.New(
					header.ACK|segment.Header.Flag,
					segment.Header.Sequence,
					nil)).SendTo(fileReceiver.senderAddr)
		}

		if compRes := segment.Header.Compare(expectedHeader); compRes == 0 {
			// Handle an inorder segment
			if fileReceiver.exitableHandleSegment(segment) {
				break loop
			}
			expectedHeader = segment.Header.NextInSequence()
			// Keep getting the next expected window from the cached window
		inner:
			for {
				if subsequent := fileReceiver.window.Get(expectedHeader); subsequent != nil {
					// Handle this segment
					if fileReceiver.exitableHandleSegment(subsequent) {
						break loop
					}
					expectedHeader = segment.Header.NextInSequence()
					// Remove from cache
					if subsequence, ok := subsequence.(Segment); ok {
						subsequence.canRemove()
					} else {
						log.Debug.Panic("wrong format for segment: expected a filerecever.Segment")
					}
				} else {
					break inner
				}
			}
		} else {
			log.Info.Printf("out of sequence packet: got %#v, expected %#v.",
				segment.Header, expectedHeader)
			// This packet is later in the sequence
			if compRes > 0 {
				log.Info.Printf("packet with header %#v cached.", segment.Header)
				// Go ahead and cache this packet (non-blocking cache)
				go fileReceiver.window.Add(segment)
			} else {
				// This packet is received already
				// compRes < 0 : packet received
				log.Info.Printf("packet with header %#v received: pass.", segment.Header)
			}

		}
	}
}

var errorExit = errors.New("exiting now")

func (fileReceiver *FileReceiver) exitableHandleSegment(segmnet *Segment) bool {
	if err := fileReceiver(segment); err != nil {
		if err == errorExit {
			return true
		}
		log.Warning.Println(err)
	}
	return false
}

func (fileReceiver *FileReceiver) handleSegment(segment *Segment) error {
	var err error
	switch {
	case segment.IsFILE():
		if fileReceiver.currentFile == nil {
			fileReceiver.currentFile, err = os.Create(string(segment.Payload))
			if err != nil {
				log.Warning.Fatalf("Unable to create file: %s", err)
			}
			go log.Info.Println("New FILE request: spawned a file reconstructing thread")
			go reconstructFile(fileReceiver.currentFile,
				fileReceiver.reconstructData,
				fileReceiver.reconstructDone)
		} else {
			// File duplication
			go log.Info.Println("duplicated FILE request: skip.")
			if fileReceiver.currentFile.Name() == string(segment.Payload) {
				return nil
			}
		}
	case segment.IsEXIT():
		if fileReceiver.currentFile != nil {
			// Send an eof signal
			sendEOF(fileReceiver.reconstructData, fileReceiver.reconstructDone)
			fileReceiver.currentFile = nil
		}
		return errorExit
	case segment.IsEOF():
		if fileReceiver.currentFile != nil {
			sendEOF(fileReceiver.reconstructData, fileReceiver.reconstructDone)
			fileReceiver.currentFile = nil
		}
	// Normal file packet
	default:
		if fileReceiver.currentFile == nil {
			log.Warning.Println("Received data packet but no file is set up.")
		} else {
			fileReceiver.reconstructData <- segment.Payload
		}
	}
	return nil
}

func sendEOF(reconstructData chan<- []byte, reconstructDone <-chan struct{}) {
	// Close the file and wait til the file is closed
	reconstructData <- nil
	<-reconstructDone
}
