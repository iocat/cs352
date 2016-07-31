package sender

import (
	"net"

	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram"
)

// Sender represents a sender that can sends data on UDP
type Sender interface {
	Broadcast()
	SendTo(*net.UDPAddr)
}

// New creates a new sender
func New(conn *net.UDPConn, packet *datagram.Segment) Sender {
	if conn == nil {
		panic("no connection is given to the sender.")
	}
	if packet == nil {
		panic("no packet is given to the sender.")
	}
	return &udpBroadcaster{
		packet: packet,
		conn:   conn,
	}
}

// udpBroadcaster is the concrete implementation of Sender
type udpBroadcaster struct {
	packet *datagram.Segment
	// The UDP Socket
	conn *net.UDPConn
}

func (sender udpBroadcaster) GoString() string {
	return sender.packet.GoString()
}

func (sender *udpBroadcaster) Broadcast() {
	sender.conn.Write(sender.packet.Bytes())
}

func (sender *udpBroadcaster) SendTo(addr *net.UDPAddr) {
	sender.conn.WriteToUDP(sender.packet.Bytes(), addr)
}
