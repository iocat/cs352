package sender

import (
	stdlog "log"
	"net"
	"time"

	"github.com/iocat/rutgers-cs352/pa2/log"
	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram"
)

// TimeoutSender represents a sender that after a certain amount of time
// resend a same packet
// To use: timeout := sender.TimeoutSender{
//      Sender: sender.New(packet),
//      Ticker: time.NewTicker(delay)
// }
// or
// timeout := sender.NewTimeout(packet, duration)
type TimeoutSender interface {
	Start(addr *net.UDPAddr)
	Stop()
}

type timeoutSender struct {
	*time.Ticker
	Sender
}

// NewTimeout creates a timeout sender given the packet and the duration that it takes
func NewTimeout(conn *net.UDPConn, packet *datagram.Segment,
	duration time.Duration) TimeoutSender {
	return &timeoutSender{
		Sender: New(conn, packet),
		Ticker: time.NewTicker(duration),
	}
}

// Start starts the timeout sender on another thread, which would periodically
// forward the packet to the given address.
// If addr is nil this method implicitly assumes the connection is a broadcast
// Non-blocking call
func (timeout *timeoutSender) Start(addr *net.UDPAddr) {
	go func(addr *net.UDPAddr) {
		var (
			l          *stdlog.Logger
			retransmit = false
			timeoutS   = ""
		)
		for _ = range timeout.Ticker.C {
			if retransmit {
				timeoutS = "timeout: re"
				l = log.Warning
			} else {
				l = log.Info
			}
			if addr == nil {
				l.Printf("%sbroadcast packet %#v", timeoutS, timeout.Sender)
				timeout.Broadcast()
			} else {
				l.Printf("%stransmit packet %#v", timeoutS, timeout.Sender)
				timeout.SendTo(addr)
			}
			retransmit = true
		}
	}(addr)
}
