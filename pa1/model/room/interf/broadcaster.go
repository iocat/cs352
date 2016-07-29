package interf

import "github.com/iocat/rutgers-cs352/pa1/model/message"

type Broadcaster interface {
	Broadcaster() chan<- message.Message
}
