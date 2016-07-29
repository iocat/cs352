// Package message represents a message that has a color
// associated with the message
// The message interface{} allows a message to have a string
// representation
package message

import (
	"fmt"

	"github.com/iocat/rutgers-cs352/pa1/model/color"
)

// Message interface allows the message to be printed out
type Message interface {
	String() string
}

// ConcreteMessage is a basic type for the user's message
type ConcreteMessage struct {
	color.Color
	string
}

// NewConcrete creates an implementation of the concrete message
func NewConcrete(col *color.Color, msg string) *ConcreteMessage {
	return &ConcreteMessage{
		Color:  *col,
		string: msg,
	}
}

// String implements the Message interface
// this method will print a message with the predefined color
func (mes *ConcreteMessage) String() string {
	return fmt.Sprintf("%s%s", mes.Color.String(), mes.string)
}
