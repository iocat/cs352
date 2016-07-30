// Package protocol is used to store global set up for the internet
// protocol
package protocol

import (
	"time"

	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram/header"
)

const (
	// SegmentTimeout is the timeout before the sender resends
	// the same packet
	SegmentTimeout = 1500 * time.Millisecond
	// SetupTimeout is the timeout before the sender stops broadcasting
	// the establishment of connection
	SetupTimeout = 10 * time.Second
	// UnresponsiveTimeout is the timeout before the sender gets rid of
	// the client because the client is not responsive to the sender packet
	UnresponsiveTimeout = 3 * time.Second

	// HeaderSize is the size of the header
	HeaderSize = header.HeaderSizeInBytes
	// PayloadSize is the size of the payload regardless of the header size
	PayloadSize = 100
	// SegmentSize is the size in bytes of the segment
	SegmentSize = HeaderSize + PayloadSize
	// WindowSize is the window size of the protocol
	WindowSize = 10
)
