package filesender

import (
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"github.com/iocat/rutgers-cs352/pa2/log"
	"github.com/iocat/rutgers-cs352/pa2/protocol"
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram"
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram/header"
	"github.com/iocat/rutgers-cs352/pa2/protocol/sender"
	"github.com/iocat/rutgers-cs352/pa2/protocol/window"
)

// FileSender maintains a stateful connection with a set of receivers
// FileSender interacts with the window to make sure every client receive
// the packets before sliding the window forward
type FileSender struct {
	Files []*os.File

	// A set of receivers that accept the file sending request from the server
	// can only be changed for each file break.
	receivers map[Addr]*Receiver
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

	// The size of the window
	WindowSize int

	// The dropping chance of the received packet
	DroppingChance int

	// The broadcasting socket
	broadcast *net.UDPConn
	// The listening socket
	listen *net.UDPConn

	newResponse chan receiverResponse

	done chan struct{}
}

// NewWithWindowSize creates a new file sender with a fixed size capacity provided
func NewWithWindowSize(size int, broadcast *net.UDPConn, listen *net.UDPConn, files []*os.File) *FileSender {
	fileSender := &FileSender{
		SegmentTimeout:      protocol.SegmentTimeout,
		SetupTimeout:        protocol.SetupTimeout,
		UnresponsiveTimeout: protocol.UnresponsiveTimeout,
		WindowSize:          size,

		newResponse: make(chan receiverResponse),
		Files:       files,
		broadcast:   broadcast,
		listen:      listen,
		done:        make(chan struct{}),
	}
	return fileSender
}

// New creates a new file sender
// Caller must provides conn, which is the broadcasting socket
func New(broadcast *net.UDPConn, listen *net.UDPConn, files []*os.File) *FileSender {
	return NewWithWindowSize(protocol.WindowSize, broadcast, listen, files)
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

func (fs *FileSender) listenResponse() {
loop:
	for {
		var data = make([]byte, header.HeaderSizeInBytes)
		// Read the packet
		size, addr, err := fs.listen.ReadFromUDP(data[0:])
		if err != nil {
			log.Warning.Println("Waiting for ACKs: error: ", err)
			break loop
		}

		if toDrop(fs.DroppingChance) {
			log.Warning.Printf("pseudo packet drop: %#v", datagram.NewFromUDPPayload(data))
			continue loop
		}

		// Check the size of the data
		if size < header.HeaderSizeInBytes {
			log.Debug.Fatalln("Waiting for ACKs: the received packet size is not valid: expected", header.HeaderSizeInBytes)
			continue
		}
		fs.newResponse <- receiverResponse{
			data: data[:size],
			addr: addr,
		}
	}
}

// Run is a blocking call that starts the sending server
func (fs *FileSender) Run() {
	if fs.DroppingChance > 100 || fs.DroppingChance < 0 {
		log.Warning.Fatalf("the dropping chance should be in the range of [0,100]")
	}
	defer fs.broadcast.Close()
	defer fs.listen.Close()
	var (
		h = header.Header{
			Flag:     header.RED,
			Sequence: 0,
		}
		toExit = false
		window = window.New(fs.WindowSize)
	)
	go fs.listenResponse()
	for i, file := range fs.Files {
		h = fs.setup(file, h)
		if i == len(fs.Files)-1 {
			toExit = true
		}
		h = fs.send(window, file, h, toExit)
		// Close the sent file
		file.Close()
	}

}

func decorate(h header.Header, flags ...header.Flag) header.Header {
	var result = h
	for _, f := range flags {
		result.Flag |= f
	}
	return result
}

func (fs *FileSender) loadFileToWindow(w *window.Window, file *os.File, start header.Header) header.Header {
	var (
		h        = start
		producer = newFileProducer(file, protocol.PayloadSize)
		segments = make([]window.Segment, 0, fs.WindowSize)
	)
	for {
		var toBreak bool
		next, err := producer.Produce()
		if err != nil {
			log.Warning.Fatalf("produce next payload: %s", err)
		}
		if len(next) == 0 {
			h.EOF()
			toBreak = true
		}
		// Create a new auto sending segment
		ts := fs.newTimeoutSegment(h, next)
		segments = append(segments, ts)
		if len(segments) == fs.WindowSize || h.IsEOF() {
			w.Load(segments)
			for _, s := range segments {
				s.(*timeoutSegment).Start(nil)
			}
			// Reset the segment list
			segments = segments[0:0]
		}
		h = h.Next()
		if toBreak {
			break
		}
	}
	return h
}

// setup sets up the broadcast address, this method continuously sends out the
// filename packet after each SegmentTimeout
// At a same time this method accepts new client with a deadline of half a second
// The setup process lasts as long as the SetupTimeout
func (fs *FileSender) setup(file *os.File, h header.Header) header.Header {
	// Reset the receiver set
	fs.receivers = make(map[Addr]*Receiver)

	log.Info.Println("SETUP: Sending an initative (FILE) packet for file name:", file.Name())
	// Start broadcasting a FILE segment
	filePacket := fs.newTimeoutSegment(decorate(h, header.FILE), []byte(file.Name()))
	filePacket.Start(nil)

	// Create setup timer
	timer := time.NewTimer(fs.SetupTimeout).C
loop:
	for {
		select {
		case response := <-fs.newResponse:
			// Check if the address existed
			if _, ok := fs.receivers[getAddr(response.addr)]; ok {
				continue
			} else if s := datagram.NewFromUDPPayload(response.data); s.IsFILE() && s.IsACK() {
				go log.Info.Println("SETUP: New client accepted: address @", response.addr.String())
				// Add the receiver to the set
				fs.receivers[getAddr(response.addr)] = NewReceiver(getAddr(response.addr), fs.UnresponsiveTimeout)
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
				filePacket.Stop()
				break loop
			}
		}
	}
	return h.Next()
}

// send starts sending the file in terms of packet
// This method makes sure all receivers received the file and maintains a
// sending window
func (fs *FileSender) send(w *window.Window, file *os.File, first header.Header, toExit bool) header.Header {
	var (
		doneReceiveACK = make(chan struct{})
		waitReceiveACK = sync.WaitGroup{}
	)
	// Start a new thread that listens to acknowlegement
	waitReceiveACK.Add(1)
	go fs.handleACK(w, doneReceiveACK, &waitReceiveACK)
	// Iteractively load file to window
	log.Info.Printf("BROADCAST: Start broadcasting file: file's name \"%s\"", file.Name())
	h := fs.loadFileToWindow(w, file, first)
	// Broadcast exit packet
	if toExit {
		exit := fs.newTimeoutSegment(decorate(h, header.EXIT), nil)
		w.Load([]window.Segment{exit})
		exit.Start(nil)
	}
	// Signaling the receiving ACK thread to stop then wait until every ACKs have been received
	close(doneReceiveACK)
	waitReceiveACK.Wait()
	if toExit {
		log.Info.Println("every receivers acknowledged. Exit.")
	}
	return h
}

type receiverResponse struct {
	data []byte
	addr *net.UDPAddr
}

// handleACK receives acknowledgement from client and marks the segment as removed
// doneReceivingSignal is a signal that asks the method to stop receiving ACK
// It does not mean receiveACK stop right away. receiveACK only stop when window
// is empty ( no more segment that needs an ACK )
func (fs *FileSender) handleACK(w *window.Window, doneReceivingSignal <-chan struct{}, wg *sync.WaitGroup) {
	var (
		unresponsiveAddr = make(chan Addr)
	)
	// Set every receivers to start tracking timeout
	for _, receiver := range fs.receivers {
		go receiver.Timeout(unresponsiveAddr)
	}
loop:
	for {
		select {
		case <-doneReceivingSignal:
			// Check empty window first before exit
			if w.Empty() {
				break loop
			}
		case response := <-fs.newResponse:
			if receiver, ok := fs.receivers[getAddr(response.addr)]; ok {
				// Reset the timer
				receiver.Reset()
				segment := w.Get(datagram.NewFromUDPPayload(response.data).Header.Pure())
				if segment == nil {
					continue
				}
				ts := segment.(*timeoutSegment)
				// Marked as ACKed
				ts.ACK(getAddr(response.addr))
				// Check if everyone acked, marks this segment as removable
				if ts.HadAllACKed(fs.receivers) {
					ts.Stop()
				}
			} else {
				log.Info.Printf("Handle ACK: Received packet from an unknown receiver @%s", response.addr.String())
			}
		case addr := <-unresponsiveAddr:
			log.Warning.Printf("client %s is unresponsive. Removed client.", addr)
			fs.receivers[addr].Stop()
			// Get rid of the receiver
			delete(fs.receivers, addr)
			if len(fs.receivers) == 0 {
				log.Warning.Fatalf("no receivers left. Exit.")
			}
		}
	}
	wg.Done()
}

// netTimeoutSegment creates an timeout segment corresponding to this FileSender
func (fs *FileSender) newTimeoutSegment(
	header header.Header,
	payload []byte) *timeoutSegment {
	return &timeoutSegment{
		segment:           datagram.NewWithHeader(header, payload),
		TimeoutSender:     sender.NewTimeout(fs.broadcast, datagram.NewWithHeader(header, payload), fs.SegmentTimeout),
		receiverACKedAddr: make(map[Addr]bool),
		done:              make(chan struct{}),
	}
}

// Done signifies the fileSender that it needs to be stopped
func (fs *FileSender) Done() {
	close(fs.done)
}
