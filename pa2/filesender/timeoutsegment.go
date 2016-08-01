package filesender

import (
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram"
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram/header"
	"github.com/iocat/rutgers-cs352/pa2/protocol/sender"
)

// timeoutSegment is a concrete implementation of window.Segment
type timeoutSegment struct {
	segment *datagram.Segment
	sender.TimeoutSender
	done chan struct{}
	// A set of acknowledgement from receivers
	receiverACKedAddr map[Addr]bool
}

// ACK marks the segment as ACKed by the given address
func (tSegment *timeoutSegment) ACK(addr Addr) {
	tSegment.receiverACKedAddr[addr] = true
}

// HadACKed checks whether the address had ACKed this segment or not
func (tSegment *timeoutSegment) HadACKed(addr Addr) bool {
	ok := tSegment.receiverACKedAddr[addr]
	return ok
}

// TODO: optimize this method O(n^2)
// HadAllACKed checks if the ACKed receivers are all in the provided set
func (tSegment *timeoutSegment) HadAllACKed(addrSet map[Addr]*Receiver) bool {
	for addr := range addrSet {
		// If the ACKed receiver set does not contin the address in the addrSet
		if ok := tSegment.receiverACKedAddr[addr]; !ok {
			return false
		}
	}
	return true
}

// Header gets the header of this TimeoutSegment
func (tSegment *timeoutSegment) Header() header.Header {
	return tSegment.segment.Header.PureHeader()
}

// Removable implements the window.Segment interface
// It returns a mark that check whether a segment is removable
func (tSegment *timeoutSegment) Removable() <-chan struct{} {
	return tSegment.done
}

// Stop marks the segment as removable and stop the sender from
// sending another package
// Stop overrides sender.TimeoutSender.Stop()
func (tSegment *timeoutSegment) Stop() {
	// Signifies the sender to remove this segment
	close(tSegment.done)
	// Stop the sender from retransmitting this packet
	tSegment.TimeoutSender.Stop()
}
