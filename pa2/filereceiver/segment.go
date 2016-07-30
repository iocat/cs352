package fileReceiver

import (
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram"
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram/header"
	"github.com/iocat/rutgers-cs352/pa2/protocol/window"
)

type Segment interface {
	window.Segment
	canRemove()
}

type receiverSegment struct {
	*datagram.Segment
	markedRemovable chan struct{}
}

func newReceiverSegment(segment *datagram.Segment) Segment {
	return &newSegment{
		markedRemovable: make(chan struct{}),
		Segment:         segment,
	}
}
func (rs *receiverSegment) Header() header.Header {
	return rs.Segment.Header.PureHeader()
}

func (rs *receiverSegment) canRemove() {
	close(rs.markedRemovable)
}

func (rs *receiverSegment) Removable() <-chan struct{} {
	return rs.markedRemovable
}
