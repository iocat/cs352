// Package user allows us to use the user model object intuitively like this
// -- Create a user that is ready to receive some messages
// usr:= user.New("name", model.Color{1,1,1},handleMethod)
// -- Receive a message from a server or whatever and send the
// -- message to the handleMethod assigned by the server
// usr.Receive(someOtherUser.Message("Hello"))
// -- Sign out and never send another message again
// -- Should be called to avoid goroutine leak
// usr.SignOut()
//
package user

import (
	"errors"
	"fmt"

	"github.com/iocat/rutgers-cs352/pa1/model/color"
	"github.com/iocat/rutgers-cs352/pa1/model/message"
)

// User interface is a chatroom user that can receives a message
// has a username, a color and an ability to sign out
type User interface {
	Color() *color.Color
	Username() string
	Message(string) message.Message
	String() string
}

// ConcreteUser represents a chat client and implements the User interface
type ConcreteUser struct {
	color    *color.Color
	username string
}

// Message is the user's Message type that decorates the ConcreteMessage
type Message struct {
	message.ConcreteMessage
	username string
}

// String implements user's Message interface this method print out
// a user message with the username decorated
func (um *Message) String() string {
	// Underline the username
	var usernameColor = color.New(color.DisplayUnderline, um.Fg(), um.Bg())
	return fmt.Sprintf("%s%s%s%s: %s%s",
		usernameColor.String(),
		um.username,
		color.Reset.String(),
		um.ConcreteMessage.Color.String(),
		um.ConcreteMessage.String(),
		color.Reset.String())
}

// Message creates a chat message which included the username
func (usr *ConcreteUser) Message(mes string) message.Message {
	return &Message{
		ConcreteMessage: *message.NewConcrete(usr.color, mes),
		username:        usr.username,
	}
}

// MessageHandler represents a function that
// receives a message and do something with it
type MessageHandler func(message.Message)

// New creates a new user, a new goroutine ( lightweight thread )
// is created to listen on the message Channel
// messageHandler optionally gives the client a chance to handle
// a new message when it is received
func New(
	username string,
	col *color.Color,
	messageHandler MessageHandler,
) (*ConcreteUser, error) {
	if len(username) > 100 {
		return nil, errors.New("username is longer than 100 characters")
	}
	usr := ConcreteUser{
		username: username,
		color:    col,
	}

	return &usr, nil
}

// Username returns the username of the user
func (usr *ConcreteUser) Username() string {
	return usr.username
}

// Color returns the color of the user
func (usr *ConcreteUser) Color() *color.Color {
	return usr.color
}

// Print the username with the predefined color
func (usr *ConcreteUser) String() string {
	return fmt.Sprintf("%s%s%s", usr.color.String(), usr.username, color.Reset.String())
}
