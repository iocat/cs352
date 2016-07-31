package filesender

import (
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/iocat/rutgers-cs352/pa2/log"
	"github.com/iocat/rutgers-cs352/pa2/protocol"
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram"
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram/header"
	"github.com/iocat/rutgers-cs352/pa2/protocol/window"
)

// FileSender maintains a stateful connection with a set of receivers
// FileSender interacts with the window to make sure every client receive
// the packets before sliding the window forward
type FileSender struct {
	// A set of receivers that accept the file sending request from the server
	// can only be changed for each file break.
	receivers map[Addr]*Receiver

	Files []*os.File

	// A timeout before sending the
	// actual file packets. This timeout makes sure that no more client
	// is added after the file packets are sent out
	SetupTimeout time.Duration

	// A timeout that waits for client response, if client does not response
	// after this timeout, the client is drop and is no longer available to
	// communicate with. Subsequent packets from this client are completely
	// ignored
	UnresponsiveTimeout time.Duration

	// The timeout for the segments which will be resent automatically
	SegmentTimeout time.Duration

	// The window used to store the TimeoutSegment
	window *window.Window

	// The broadcasting socket
	broadcast *net.UDPConn

	listen *net.UDPConn

	done chan struct{}
}

// New creates a new file sender
// Caller must provides conn, which is the broadcasting socket
func New(broadcast *net.UDPConn, listen *net.UDPConn, files []*os.File) *FileSender {
	fileSender := &FileSender{
		SegmentTimeout:      protocol.SegmentTimeout,
		SetupTimeout:        protocol.SetupTimeout,
		UnresponsiveTimeout: protocol.UnresponsiveTimeout,

		window:    window.New(protocol.WindowSize),
		Files:     files,
		broadcast: broadcast,
		listen:    listen,
		done:      make(chan struct{}),
	}
	return fileSender
}

// Run is a blocking call that starts the sending server
func (fs *FileSender) Run() {
	defer fs.broadcast.Close()
	defer fs.listen.Close()
	h := header.Header{
		Flag:     header.RED,
		Sequence: 0,
	}
loop:
	for _, file := range fs.Files {
		select {
		case <-fs.done:
			fs.exit()
			break loop
		default:
			h = fs.setup(file, h)
			h = fs.send(file, h)
			// Close the sent file
			file.Close()
		}
	}
	fs.exit()
}

func (fs *FileSender) exit() {
	// Broadcast an EXIT Segment
	log.Info.Println("Send an EXIT message to receivers.")
	fs.broadcast.Write(datagram.New(header.EXIT, 0, nil).Bytes())
}

// send starts sending the file in terms of packet
// This method makes sure all receivers received the file and maintains a
// sending window
func (fs *FileSender) send(file *os.File, first header.Header) header.Header {
	var (
		// the current header
		h              = first
		doneReceiveACK = make(chan struct{})
		waitReceiveACK sync.WaitGroup
		// closure to get the next payload... omg, this is magic @@
		next = func(file *os.File) []byte {
			return nextPayload(file, protocol.PayloadSize)
		}
		// A function to broadcast payload with a header
		broadcastSegmentWithTimeout = func(h header.Header, payload []byte) {
			// Create a new auto sent segment
			timeoutSegment := fs.newTimeoutSegment(h, payload)
			// start broadcasting this segment
			timeoutSegment.Start(nil)
			// Add the segment to the window
			log.Debug.Printf("BROADCAST: Add segment to the window: segment header: %#v", timeoutSegment.Header())
			fs.window.Add(timeoutSegment)
		}
	)
	waitReceiveACK.Add(1)
	// Start a new thread that listens to acknowlegement
	go fs.handleACK(doneReceiveACK, &waitReceiveACK)
	log.Info.Printf("BROADCAST: Start broadcasting file: file's name \"%s\"", file.Name())
	for payload := next(file); len(payload) != 0; payload = next(file) {
		broadcastSegmentWithTimeout(h, payload)
		// Get the next header in the sequence
		h = h.Next()
	}
	// Set the header to EOF
	_ = h.EOF()
	broadcastSegmentWithTimeout(h, nil)
	close(doneReceiveACK)
	// Wait until everyone acknowledged the packets
	waitReceiveACK.Wait()
	// Return the next header in sequence
	return h.Next()
}

type receiverResponse struct {
	data []byte
	addr *net.UDPAddr
}

func (fs *FileSender) listenResponse(doneListen <-chan struct{},
	newResponse chan<- receiverResponse, wg *sync.WaitGroup) {
	log.Debug.Println("Waiting for ACKs: Start receiving ACK reponses")
loop:
	for {
		select {
		case <-doneListen:
			break loop
		default:
			// RECEIVE ACKed HEADER segment FROM RECEIVERS
			var data = make([]byte, header.HeaderSizeInBytes)
			// Set the read deadline to the unresponsive time
			fs.listen.SetReadDeadline(
				time.Now().Add(fs.UnresponsiveTimeout))
			// Read the packet
			size, addr, err := fs.listen.ReadFromUDP(data[0:])
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					log.Warning.Println("Waiting for ACKs: timeout.")
					break
				}
				log.Warning.Println("Waiting for ACKs: error: ", err)
				break
			}
			// Check the size of the data
			if size < header.HeaderSizeInBytes {
				log.Debug.Fatalln("Waiting for ACKs: the received packet size is not valid: expected",
					header.HeaderSizeInBytes)
				continue
			}
			newResponse <- receiverResponse{
				data: data[:size],
				addr: addr,
			}
		}
	}
	wg.Done()
	log.Debug.Println("Waiting for ACKs: Stopped receiving ACK responses.")
}

// handleACK receives acknowledgement from client
// and marks the segment as removed
// doneReceivingSignal is a signal that asks the method to stop receiving ACK
// It does not mean receiveACK stop right away. receiveACK only stop when window
// is empty ( no more segment that needs an ACK )
func (fs *FileSender) handleACK(doneReceivingSignal <-chan struct{},
	wg *sync.WaitGroup) {

	var (
		unresponsiveAddr = make(chan Addr)

		listenResponseWait sync.WaitGroup
		doneListenResponse = make(chan struct{})
		newResponse        = make(chan receiverResponse)
	)
	listenResponseWait.Add(1)
	go fs.listenResponse(doneListenResponse, newResponse, &listenResponseWait)
	// Set every receivers to start tracking timeout
	for _, receiver := range fs.receivers {
		go receiver.Timeout(unresponsiveAddr)
	}
loop:
	for {
		select {
		case <-doneReceivingSignal:
			// Check empty window first before exit
			if fs.window.IsEmpty() {
				// Stop receiving response
				close(doneListenResponse)
				// Wait till the listenResponse process closes
				listenResponseWait.Wait()
				break loop
			}
		case response := <-newResponse:
			if receiver, ok := fs.receivers[getAddr(response.addr)]; ok {
				// Reset the timer
				receiver.Reset()
				segment := fs.window.Get(datagram.NewFromUDPPayload(response.data).Header)
				if segment, ok := segment.(*timeoutSegment); ok {
					// Marked as ACKed
					segment.ACK(getAddr(response.addr))
					// Check if everyone acked, marks this segment as removable
					if segment.HadAllACKed(fs.receivers) {
						segment.Stop()
					}
				} else {
					log.Debug.Fatalf("Handle ACK: Invalid segment type in the window. Got %T, expected: *timeoutSegment", segment)
				}
			} else {
				log.Info.Printf("Handle ACK: Received packet from an unknown receiver @%s", response.addr.String())
			}
		case addr := <-unresponsiveAddr:
			// Get rid of the receiver
			delete(fs.receivers, addr)
		}
	}
	wg.Done()
}

// nextPayload gets the next payload, the payload contains
// an exact number of byte that will be sent
// bufferSize is the maximum size of the buffer
// file is the file to be read
func nextPayload(file *os.File, bufferSize int) []byte {
	var (
		payload    = make([]byte, bufferSize)
		readLength int
		err        error
	)
	if readLength, err = file.Read(payload); err != nil {
		if err == io.EOF {
			return nil
		}
		log.Warning.Println("Getting next payload: error: reading file: ", err)
	}
	return payload[:readLength]
}

// setup sets up the broadcast address, this method continuously sends out the
// filename packet after each SegmentTimeout
// At a same time this method accepts new client with a deadline of half a second
// The setup process lasts as long as the SetupTimeout
func (fs *FileSender) setup(file *os.File, h header.Header) header.Header {
	// Reset the receiver set
	fs.receivers = make(map[Addr]*Receiver)

	log.Info.Println("SETUP: Sending an initative (FILE) packet for file name:", file.Name())
	var (
		timeoutSegment *timeoutSegment

		newResponse  = make(chan receiverResponse)
		responseDone = make(chan struct{})
		responseWait sync.WaitGroup
	)
	responseWait.Add(1)
	// listen to client response
	go fs.listenResponse(responseDone, newResponse, &responseWait)

	// Start sending timeout segment
	timeoutSegment = fs.newTimeoutSegment(
		header.Header{
			Flag:     header.FILE | h.Flag,
			Sequence: h.Sequence,
		}, []byte(file.Name()))
	// Start broadcasting the segment periodically
	timeoutSegment.Start(nil)
	// Create a timer
	timer := time.NewTimer(fs.SetupTimeout).C
loop:
	for {
		select {
		case response := <-newResponse:
			if _, ok := fs.receivers[getAddr(response.addr)]; ok {
				continue
			} else {
				go log.Info.Println("SETUP: New client accepted: address @", response.addr.String())
				// Add the receiver to the set
				fs.receivers[getAddr(response.addr)] =
					NewReceiver(getAddr(response.addr), fs.UnresponsiveTimeout)
			}
		case <-timer:
			// No receiver: keep setting up
			if len(fs.receivers) == 0 {
				go log.Info.Println("SETUP: no receiver: keep waiting for new connections.")
				// Reset the timer
				timer = time.NewTimer(fs.SetupTimeout).C
			} else {
				go func() {
					log.Info.Println("SETUP: TIMEOUT & FINISH. Start sending file to the following client(s): ")
					for rc := range fs.receivers {
						log.Info.Printf("\t%s", rc)
					}
				}()
				// Stop broadcasting the filename
				timeoutSegment.Stop()
				// Close then wait till the response receiver is done
				close(responseDone)
				responseWait.Wait()
				break loop
			}
		}
	}
	return h.Next()
}

// netTimeoutSegment creates an timeout segment corresponding to this FileSender
func (fs *FileSender) newTimeoutSegment(
	header header.Header,
	payload []byte) *timeoutSegment {
	return newTimeoutSegment(
		datagram.NewWithHeader(header, payload),
		fs.broadcast,
		fs.SegmentTimeout,
	)
}

// Done signifies the fileSender that it needs to be stopped
func (fs *FileSender) Done() {
	close(fs.done)
}
