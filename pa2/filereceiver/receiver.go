package filereceiver

import (
	"errors"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/iocat/rutgers-cs352/pa2/log"
	"github.com/iocat/rutgers-cs352/pa2/protocol"
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram"
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram/header"
	"github.com/iocat/rutgers-cs352/pa2/protocol/sender"
)

// FileReceiver represents a receiver that receives file packets from
// UDP Connection
type FileReceiver struct {
	senderTimeout time.Duration
	senderAddr    *net.UDPAddr

	droppingChance int

	// current header is the current header of the latest packet
	currentHeader header.Header

	// The socket that this FileReceiver uses to send and replies to
	// the broadcaster
	socket *net.UDPConn

	// The port to replies back to the sender
	senderPort int

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
func New(outputDir string, conn *net.UDPConn, droppingChance int, senderPort int) *FileReceiver {
	if droppingChance < 0 || droppingChance > 100 {
		log.Warning.Fatal("dropping chance out of range: should be between 0 and 100")
	}
	if err := createDir(outputDir); err != nil {
		log.Warning.Fatalf("cannot create %s: %s", outputDir, err)
	}
	return &FileReceiver{
		socket:          conn,
		reconstructData: make(chan []byte),
		reconstructDone: make(chan struct{}),
		senderPort:      senderPort,
		droppingChance:  droppingChance,
		out:             outputDir,
		senderTimeout:   protocol.UnresponsiveTimeout,
	}
}

func (fr *FileReceiver) switchSenderAddrPort() {
	fr.senderAddr.Port = fr.senderPort
}

// receiveData receives data buffer and send it to the newData channel
// receiveData knows nothing about the data packet it receives. It makes sure
// the received packets is from the acknowledged sender
func (fr *FileReceiver) receiveData(newData chan<- []byte, hasTimeout chan<- struct{}) {
	var (
		data []byte
	)
loop:
	for {
		data = make([]byte, protocol.SegmentSize)
		fr.socket.SetReadDeadline(time.Now().Add(fr.senderTimeout))
		length, addr, err := fr.socket.ReadFromUDP(data)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				hasTimeout <- struct{}{}
				break loop
			}
			log.Warning.Printf("receive data error: %s", err)
		}
		if fr.senderAddr == nil {
			log.Info.Printf("new sender dectected: set this %s as an official sender", addr.String())
			fr.senderAddr = addr
			fr.switchSenderAddrPort()
		}
		// Check sender address
		if addr.IP.String() != fr.senderAddr.IP.String() {
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
func init() {
	rand.Seed(time.Now().UnixNano())
}

func toDrop(droppingChance int) bool {
	drop := rand.Intn(100)
	if drop < droppingChance {
		return true
	}
	return false
}

// ACK sends an ACK back to the sender
func (fr *FileReceiver) acknowledge(segment *datagram.Segment) {
	// Accepted: Send an ACK back
	sender.New(fr.socket,
		datagram.New(header.ACK|segment.Header.Flag,
			segment.Header.Sequence, nil)).SendTo(fr.senderAddr)
}

// Cache is the cache memory for the received packet
type Cache map[header.Header]*receiverSegment

// Cache cache the packet
func (c Cache) Cache(h header.Header, s *receiverSegment) {
	c[h.Pure()] = s
}

// Get gets the packet from the cache
func (c Cache) Get(h header.Header) (*receiverSegment, bool) {
	if s, ok := c[h.Pure()]; ok {
		return s, ok
	}
	return nil, false
}

// Delete deletes one packet from the cache
func (c Cache) Delete(h header.Header) {
	delete(c, h.Pure())
}

// ReceiveFiles starts receiving files
// outputDir is the directory to write the downloaded files to
func (fr *FileReceiver) ReceiveFiles() {
	var (
		newData    = make(chan []byte)
		hasTimeout = make(chan struct{})
		cache      = make(Cache)

		expectedHeader = header.Header{
			Flag:     header.RED,
			Sequence: 0,
		}
	)
	go fr.receiveData(newData, hasTimeout)
loop:
	for {
		select {
		case <-hasTimeout:
			log.Warning.Printf("Sender is unresponsive or sender does not exist. Exit.")
			break loop
		case data := <-newData:
			segment := newReceiverSegment(datagram.NewFromUDPPayload(data))
			// try to drop the packet
			if toDrop(fr.droppingChance) {
				log.Warning.Printf("pseudo packet drop: drop one with header: %#v", segment.Header())
				break
			}
			fr.acknowledge(segment.Segment)

			if compRes := segment.Header().Compare(expectedHeader); compRes == 0 {
				// Handle an inorder segment
				if fr.exitableHandleSegment(segment) {
					break loop
				}
				expectedHeader = segment.Header().Next()
				// Keep getting the next expected window from the cache
			inner:
				for {
					if subsequent, ok := cache.Get(expectedHeader); ok {
						// Handle this segment
						if fr.exitableHandleSegment(subsequent) {
							break loop
						}
						// Remove from cache
						cache.Delete(expectedHeader)
						expectedHeader = subsequent.Header().Next()
					} else {
						// Does not have the next one
						break inner
					}
				}
			} else if compRes > 0 {
				// New file rather than a previous file
				// Reprocess the same file with the expected header is this packet
				if segment.IsFILE() {
					expectedHeader = segment.Header()
					go func(data []byte) { newData <- data }(data)
					break
				}
				// Check if this segment was cached or not
				if _, ok := cache.Get(segment.Header()); !ok {
					// Go ahead and cache this packet if possible (non-blocking cache)
					cache.Cache(segment.Header(), segment)
				}
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
	log.Info.Printf("handle packet: %#v\r", segment)
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
			go log.Info.Printf("New FILE %s: spawned a file reconstructing thread", fp)
			go reconstructFile(fr.currentFile, fr.reconstructData, fr.reconstructDone)
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
			fr.reconstructData <- []byte{}
			<-fr.reconstructDone
			fr.currentFile = nil
		}
		return errorExit
	case segment.IsEOF():
		if fr.currentFile != nil {
			fr.reconstructData <- []byte{}
			<-fr.reconstructDone
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
