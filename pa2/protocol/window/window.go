package window

import (
	"github.com/iocat/rutgers-cs352/pa2/log"

	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram/header"
)

// Segment represents the segment which has a header
// And can signifies the window to remove it from the window
type Segment interface {
	Header() header.Header
	// Removable lets the window know whether this segment is ready to
	// be removed from the window
	Removable() <-chan struct{}
}

// Window represents a fixed size auto sliding window that has no duplicated
// header
// Window however does not keep track the sequencing of packet, it just stores
// packet and controls the size of the window
type Window struct {
	maxSize int

	// A counter channel that acts like a semaphore
	// which limits the window size
	segmentCount chan struct{}

	// A channel that listens to the next segment to be added
	segmentAdder   chan Segment
	segmentRemover chan header.Header

	// A store of segments using the segment header
	segments map[header.Header]Segment

	// A get pairs of channel to implement synchronous read on map
	getRequest  chan header.Header
	getResponse chan Segment

	emptyCheck chan chan bool

	head header.Header

	done chan struct{}
}

// Add adds a new segment to the window
// This method blocks if the window is full, if there are empty spaces,
// This method returns right away and add the segment to the window.
// If header duplication occurs the method panics
func (window *Window) Add(segment Segment) {
	// Lock if the map is full
	window.segmentCount <- struct{}{}
	// Send the segment to the adder
	window.segmentAdder <- segment
}

func (window *Window) add(segment Segment) {
	if duplicated, ok := window.segments[segment.Header()]; ok {
		log.Warning.Fatalf("duplicated header: the collided segment is %#v", duplicated)
	}
	window.segments[segment.Header()] = segment
	// Start a new thread that waits for a removal signal
	go func(window *Window, h header.Header) {
		select {
		case <-window.done:
			break
		case <-segment.Removable():
			window.segmentRemover <- h
		}
	}(window, segment.Header())
}

// Get gets the segment using the header
// This method holds and waits until the segment is returned
// Get might returns nil if no segment corresponds to the provided header is in
// the window
func (window *Window) Get(h header.Header) Segment {
	window.getRequest <- h
	return <-window.getResponse
}

// IsEmpty checks whether the window is empty at the time of the check
// This call will block until the result is retrieved
func (window *Window) IsEmpty() bool {
	checkChan := make(chan bool)
	window.emptyCheck <- checkChan
	return <-checkChan
}

// get gets the segments
func (window *Window) get(h header.Header) {
	window.getResponse <- window.segments[h]
}

// remove removes the segment using the header
func (window *Window) remove(h header.Header) {
	delete(window.segments, h)
	// Release a segment that allows the window to add more segment
	<-window.segmentCount
}

// New creates a new window with max size
// + size is the size of the window
func New(size int) *Window {
	window := Window{
		maxSize:        size,
		segmentCount:   make(chan struct{}, size),
		segmentAdder:   make(chan Segment),
		segmentRemover: make(chan header.Header),
		getRequest:     make(chan header.Header),
		getResponse:    make(chan Segment),
		segments:       make(map[header.Header]Segment),
		emptyCheck:     make(chan chan bool),
		done:           make(chan struct{}),
	}
	go func(window *Window) {
	loop:
		for {
			select {
			case checkChan := <-window.emptyCheck:
				checkChan <- (len(window.segments) == 0)
			case h := <-window.segmentRemover:
				window.remove(h)
			case segment := <-window.segmentAdder:
				window.add(segment)
			case h := <-window.getRequest:
				window.get(h)
			case <-window.done:
				break loop
			}
		}
	}(&window)
	return &window
}
