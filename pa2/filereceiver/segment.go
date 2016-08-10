package filereceiver

import (
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram"
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram/header"
)

type receiverSegment struct {
	*datagram.Segment
}

func newReceiverSegment(segment *datagram.Segment) *receiverSegment {
	return &receiverSegment{
		Segment: segment,
	}
}
func (rs *receiverSegment) Header() header.Header {
	return rs.Segment.Header
}
