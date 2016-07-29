package filesender

import "time"

// Receiver represents a receiver with timeout information
// associated
type Receiver struct {
	Addr
	reset   chan struct{}
	timeout time.Duration
	stop    chan struct{}
}

// Reset resets the receiver timer
func (receiver *Receiver) Reset() {
	receiver.reset <- struct{}{}
}

// Stop stops the timeout lock
func (receiver *Receiver) Stop() {
	receiver.stop <- struct{}{}
}

// NewReceiver creates a new receiver
// This
func NewReceiver(address Addr, timeout time.Duration) *Receiver {
	receiver := &Receiver{
		reset:   make(chan struct{}),
		stop:    make(chan struct{}),
		timeout: timeout,
		Addr:    address,
	}
	return receiver

}

// Timeout starts a timeout listening on the current thread
// usage: go receiver.Timeout(signifier)
func (receiver *Receiver) Timeout(timeoutSignifier chan<- Addr) {
loop:
	for {
		select {
		// After a timeout, sends the address to the signifier
		case <-time.After(receiver.timeout):
			timeoutSignifier <- receiver.Addr
		// If receives a reset message reset the timer
		case <-receiver.reset:
			break
		case <-receiver.stop:
			break loop
		}
	}
}
