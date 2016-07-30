package datagram

import (
	"fmt"

	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram/header"
)

// A Segment represents a datagram packet on UDP
type Segment struct {
	header.Header
	Payload []byte
}

// New creates a new segment with a flag, a sequence number and a payload
func New(flag header.Flag, sequence header.Sequence, payload []byte) *Segment {
	return &Segment{
		Header: header.Header{
			Flag:     flag,
			Sequence: sequence,
		},
		Payload: payload,
	}
}

// NewWithHeader creates a new segment with the provided header and the
// payload
func NewWithHeader(head header.Header, payload []byte) *Segment {
	return &Segment{
		Header:  head,
		Payload: payload,
	}
}

// NewFromUDPPayload reads the payload and reconstruct a segment with
// a header from it.
// The first five bytes are always the header.
// The subsequent bytes become the actual payload. Any attempt to provide
// a-less-than-5-byte-long payload ends up with a panic.
func NewFromUDPPayload(payload []byte) *Segment {
	return &Segment{
		Header:  *header.NewFromBytes(payload[:5]),
		Payload: payload[5:],
	}
}

// Bytes returns the byte representation of the segment
func (segment *Segment) Bytes() []byte {
	var (
		header       = segment.Header.Bytes()
		segmentBytes = make([]byte, len(header)+len(segment.Payload))
	)
	copy(segmentBytes[0:len(header)], header)
	copy(segmentBytes[len(header):], segment.Payload)
	return segmentBytes
}

// GoString returns the string representation that is shown by fmt
func (segment Segment) GoString() string {
	// DEPRECATED min function
	var _ = func(x, y int) int {
		if x < y {
			return x
		}
		return y
	}
	return fmt.Sprintf("%s, payload size: %d byte(s)",
		segment.Header.GoString(),
		len(segment.Payload))
}
