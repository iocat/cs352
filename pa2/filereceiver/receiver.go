package filereceiver

import (
	"errors"
	"math/rand"
	"net"
	"os"
	"path/filepath"

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

	out string
}

func createDir(outputDir string) error {
	var (
		err error
	)
	if _, err = os.Stat(outputDir); err != nil {
		if os.IsNotExist(err) {
			if err = os.Mkdir(outputDir, 0711); err != nil {
				return err
			}
		}
		return err
	}
	return nil
}

// New creates a new FileReceiver object
func New(outputDir string, conn *net.UDPConn, droppingChance int) *FileReceiver {
	if droppingChance < 0 || droppingChance > 100 {
		log.Warning.Fatal("dropping chance out of range: should be between 0 and 100")
	}
	if err := createDir(outputDir); err != nil {
		log.Warning.Fatalf("cannot create %s: %s", outputDir, err)
	}
	return &FileReceiver{
		UDPConn:         conn,
		window:          window.New(protocol.WindowSize),
		reconstructData: make(chan []byte),
		reconstructDone: make(chan struct{}),
		out:             outputDir,
	}
}

// receiveData receives data buffer and send it to the newData channel
func (fr *FileReceiver) receiveData(
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
			length, addr, err := fr.ReadFrom(data)
			if err != nil {
				log.Warning.Printf("receive data error: %s", err)
			}
			if fr.senderAddr == nil {
				log.Info.Println("new sender dectected: set this sender as an official sender")
				fr.senderAddr = addr
			}
			// Check sender address
			if addr.String() != fr.senderAddr.String() {
				log.Warning.Printf("receive broadcast packet from unknown sender host, got %s, expected %s",
					addr.String(), fr.senderAddr.String())
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

func toDrop(droppingChance int) bool {
	return rand.Intn(100) < droppingChance
}

// ReceiveFiles starts receiving files
// outputDir is the directory to write the downloaded files to
func (fr *FileReceiver) ReceiveFiles() {
	var (
		newData       = make(chan []byte)
		stopReceiving = make(chan struct{})

		expectedHeader = header.Header{
			Flag:     header.RED,
			Sequence: 0,
		}
	)
	go fr.receiveData(newData, stopReceiving)
loop:
	for data := range newData {
		segment := newReceiverSegment(datagram.NewFromUDPPayload(data))
		// try to drop the packet
		if toDrop(fr.droppingChance) {
			log.Warning.Printf("pseudo packet drop: drop one with header: %#v",
				segment.Header())
			continue
		} else {
			log.Info.Printf("received %#v: send an ACK back.", segment.Header())
			// Accepted: Send an ACK back
			go sender.New(fr.UDPConn,
				datagram.New(
					header.ACK|segment.Header().Flag,
					segment.Header().Sequence,
					nil)).SendTo(fr.senderAddr)
		}

		if compRes := segment.Header().Compare(expectedHeader); compRes == 0 {
			// Handle an inorder segment
			if fr.exitableHandleSegment(segment) {
				break loop
			}
			expectedHeader = segment.Header().Next()
			// Keep getting the next expected window from the cached window
		inner:
			for {
				if subsequent := fr.window.Get(expectedHeader); subsequent != nil {
					// Handle this segment
					if fr.exitableHandleSegment(subsequent.(*receiverSegment)) {
						break loop
					}
					expectedHeader = subsequent.Header().Next()
					// Remove from cache
					if subsequent, ok := subsequent.(*receiverSegment); ok {
						subsequent.canRemove()
					} else {
						log.Debug.Panic("wrong format for segment: expected a filerecever.Segment")
					}
				} else {
					break inner
				}
			}
		} else {
			log.Info.Printf("out of sequence packet: got %#v, expected %#v.",
				segment.Header(), expectedHeader)
			// This packet is later in the sequence
			if compRes > 0 {
				log.Info.Printf("packet with header %#v cached.", segment.Header())
				// Go ahead and cache this packet (non-blocking cache)
				go fr.window.Add(segment)
			} else {
				// This packet is received already
				// compRes < 0 : packet received
				log.Info.Printf("packet with header %#v received: pass.", segment.Header())
			}

		}
	}
}

var errorExit = errors.New("exiting now")

func (fr *FileReceiver) exitableHandleSegment(segment *receiverSegment) bool {
	if err := fr.handleSegment(segment); err != nil {
		if err == errorExit {
			return true
		}
		log.Warning.Println(err)
	}
	return false
}

func (fr *FileReceiver) handleSegment(segment *receiverSegment) error {
	var err error
	switch {
	case segment.IsFILE():
		if fr.currentFile == nil {
			fp := string(segment.Payload)
			fp = filepath.Join(fr.out, filepath.Base(fp))
			fr.currentFile, err = os.Create(fp)
			if err != nil {
				log.Warning.Fatalf("Unable to create file: %s", err)
			}
			go log.Info.Println("New FILE request: spawned a file reconstructing thread")
			go reconstructFile(fr.currentFile,
				fr.reconstructData,
				fr.reconstructDone)
		} else {
			// File duplication
			go log.Info.Println("duplicated FILE request: skip.")
			if fr.currentFile.Name() == string(segment.Payload) {
				return nil
			}
		}
	case segment.IsEXIT():
		if fr.currentFile != nil {
			// Send an eof signal
			sendEOF(fr.reconstructData, fr.reconstructDone)
			fr.currentFile = nil
		}
		return errorExit
	case segment.IsEOF():
		if fr.currentFile != nil {
			sendEOF(fr.reconstructData, fr.reconstructDone)
			fr.currentFile = nil
		}
	// Normal file packet
	default:
		if fr.currentFile == nil {
			log.Warning.Println("Received data packet but no file is set up.")
		} else {
			fr.reconstructData <- segment.Payload
		}
	}
	return nil
}

func sendEOF(reconstructData chan<- []byte, reconstructDone <-chan struct{}) {
	// Close the file and wait til the file is closed
	reconstructData <- nil
	<-reconstructDone
}
