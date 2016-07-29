package interf

import (
	"github.com/iocat/rutgers-cs352/pa1/model/message"
)

type User interface {
	Error() chan<- error
	Receive(message.Message)
	Username() string
	// Dynamically change the message dispatcher
	SetBroadcaster(Broadcaster)
	// String() prints username with style
	String() string
}
