package window

import (
	"errors"
	"fmt"
	"sync"

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
type Window struct {
	// lock protects segments and count
	lock     *sync.RWMutex
	segments []Segment

	// count counts the number of segments in window
	count int

	// loadable is a signal that notifies when the window is loadable
	loadable chan struct{}
}

// Head gets the segment at first index
func (w *Window) Head() Segment {
	w.lock.RLock()
	defer w.lock.RUnlock()
	return w.segments[0]
}

// Empty checks whether the window is loadable or not
func (w *Window) Empty() bool {
	return len(w.loadable) == 0
}

// GoString returns a string representation of the window
func (w Window) GoString() string {
	var res string
	res += "Window["
	for i := 0; i < w.count; i++ {
		res += fmt.Sprintf(" %#v ", w.segments[i].Header())
	}
	res += "]"
	return res
}

// Load loads a bunch of packets into the window by mapping from the 0 index
func (w *Window) Load(segments []Segment) {
	w.loadable <- struct{}{}
	if len(segments) > len(w.segments) {
		panic(errors.New("load: parameter has wrong size"))
	}
	w.lock.Lock()
	defer w.lock.Unlock()
	w.count = copy(w.segments, segments)
	var justRemove = make(chan struct{})
	go func(justRemove <-chan struct{}, maxRemove int) {
		var removeCount = 0
		for _ = range justRemove {
			removeCount++
			if removeCount == maxRemove {
				// Allow loading of the next set
				<-w.loadable
				break
			}
		}
	}(justRemove, w.count)
	for _, segment := range segments {
		go func(justRemove chan<- struct{}, segment Segment) {
			select {
			case <-segment.Removable():
				// Notify removable
				justRemove <- struct{}{}
			}
		}(justRemove, segment)
	}
}

func distance(a header.Header, b header.Header) int {
	if (a.IsRED() && b.IsRED()) || (a.IsBLUE() && b.IsBLUE()) {
		return int(b.Sequence - a.Sequence)
	}
	return int((header.MaxSequence - a.Sequence) + b.Sequence + 1)
}

// Get gets a segment corresponding to the header
func (w *Window) Get(s header.Header) Segment {
	w.lock.RLock()
	defer w.lock.RUnlock()
	if w.count == 0 {
		return nil
	}
	var head = w.segments[0]
	// Before the sequence
	if comp := s.Compare(head.Header()); comp < 0 {
		return nil
	} else if comp == 0 {
		return head
	} else {
		d := distance(head.Header(), s)
		if d >= w.count {
			return nil
		}
		return w.segments[d]
	}
}

// New creates a new window with max size
// + size is the size of the window
func New(size int) *Window {
	return &Window{
		lock:     &sync.RWMutex{},
		segments: make([]Segment, size),
		loadable: make(chan struct{}, 1),
		count:    0,
	}
}
