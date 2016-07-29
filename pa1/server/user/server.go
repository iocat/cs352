// Package user handles the interaction between the user
// and the chatrooms
package user

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/iocat/rutgers-cs352/pa1/command"
	"github.com/iocat/rutgers-cs352/pa1/model/color"
	"github.com/iocat/rutgers-cs352/pa1/model/message"
	room "github.com/iocat/rutgers-cs352/pa1/model/room/interf"
	"github.com/iocat/rutgers-cs352/pa1/model/room/private"
	"github.com/iocat/rutgers-cs352/pa1/model/room/public"
	"github.com/iocat/rutgers-cs352/pa1/model/user"
	"github.com/iocat/rutgers-cs352/pa1/result"
)

// User represents a user that is on the server
// When the user is created 2 threads are created:
// one would handle client request
// one would send client response
// server.User implements the chatroom.User interface{}
// server.User also wraps around model.User
type User struct {
	user.User
	net.Conn
	encoder *gob.Encoder
	decoder *gob.Decoder

	errChan chan error

	// public is the room where the user stays forever
	public *public.Room
	// broadcaster is the room that this user sends the message to everyone in it
	broadcaster room.Broadcaster
	// receives message than sends to the broadcaster
	broadcastMessageChan chan message.Message
	// messageChan is the message channel this user receives message from
	messageChan chan message.Message
	// receiveBroadcaster receives a new broadcaster that this user wants
	// to receive message from
	receiveBroadcaster chan room.Broadcaster
	removeUserRequest  chan *sync.WaitGroup
	getUserRequest     chan *userGetter
	createRequest      chan *userCreator

	done chan struct{}
}

type userCreator struct {
	err error
	sync.WaitGroup
	username string
}

type userGetter struct {
	sync.WaitGroup
	user.User
}

func (su *User) Done() <-chan struct{} {
	return su.done
}

func (su *User) Error() chan<- error {
	return su.errChan
}

// Receive receives a message
func (su *User) Receive(mes message.Message) {
	su.messageChan <- mes
}

// SetBroadcaster sets the Broadcaster of the user
func (su *User) SetBroadcaster(broadcaster room.Broadcaster) {
	su.receiveBroadcaster <- broadcaster
}

// New creates a new user, adds the user to the public room, and spawns
// subroutines that actively communicates with clients
func New(pub *public.Room, conn net.Conn) (*User, error) {
	var (
		newUser user.User
		com     *command.Command
		res     *result.Result
		err     error
		encoder = gob.NewEncoder(conn)
		decoder = gob.NewDecoder(conn)
	)

	com = &command.Command{}
	decoder.Decode(com)
	// Not an expected command
	if com.Ctype != command.Create {
		err = errors.New("fail to create a new user: invalid command type")
		res = result.New(result.Failure, err.Error())
		encoder.Encode(res)
		return nil, err
	} else if len(com.Args) == 0 {
		err = errors.New("fail to create a new user: no username is received")
		res = result.New(result.Failure, err.Error())
		encoder.Encode(res)
		return nil, err
	}
	serverUser := &User{
		User:        newUser,
		Conn:        conn,
		encoder:     encoder,
		decoder:     decoder,
		public:      pub,
		broadcaster: pub,

		messageChan:          make(chan message.Message),
		receiveBroadcaster:   make(chan room.Broadcaster),
		broadcastMessageChan: make(chan message.Message),
		errChan:              make(chan error),
		done:                 make(chan struct{}),
		removeUserRequest:    make(chan *sync.WaitGroup),
		getUserRequest:       make(chan *userGetter),
		createRequest:        make(chan *userCreator),
	}
	go serverUser.synchronizeMessage()
	go serverUser.receiveError()
	go serverUser.receiveCommand()
	return serverUser, nil
}

// Receive operation error and send it to socket
func (su *User) receiveError() {
loop:
	for {
		select {
		case <-su.done:
			break loop
		case err := <-su.errChan:
			var res *result.Result
			if err == public.ErrDuplicateUsername {
				res = result.New(result.Failure, err.Error())
				var wg sync.WaitGroup
				wg.Add(1)
				su.removeUserRequest <- &wg
				wg.Wait()
			} else {
				res = result.New(result.Failure, err.Error())
			}

			if err := su.encoder.Encode(res); err != nil {
				su.handleCommunicationError("receive error", err)
				break loop
			}
		}
	}
}

// synchronizeMessage controls the concurrent access to the user message queue
// the user can only either receives a message or set a new broadcaster (that
// sends the message to user) at one particular moment in real time
func (su *User) synchronizeMessage() {
loop:
	for {
		select {
		case <-su.done:
			break loop
		case mes := <-su.messageChan:
			su.handleMessage(mes)
		case broadcaster := <-su.receiveBroadcaster:
			su.broadcaster = broadcaster
		case mes := <-su.broadcastMessageChan:
			su.broadcaster.Broadcaster() <- mes
		case remover := <-su.removeUserRequest:
			su.User = nil
			remover.Done()
		case getter := <-su.getUserRequest:
			getter.User = su.User
			getter.Done()
		case req := <-su.createRequest:
			// Create a new user from user model
			var err error
			if su.User, err = user.New(req.username, color.Randomize(), nil); err != nil {
				req.err = err
			}
			req.WaitGroup.Done()
		}
	}
}

// handleMessage is a callback that is passed down to the user
// whenever a Receive is called handleMessage is invoked
func (su *User) handleMessage(mes message.Message) {
	res := result.New(result.Message, mes.String())
	// send the result
	if err := su.encoder.Encode(res); err != nil {
		if err != nil {
			su.handleCommunicationError("sends results to the clients", err)
		}
	}
}

// receiveCommand waits til a command is completely read from a socket
func (su *User) receiveCommand() {
loop:
	for {
		select {
		case <-su.done:
			break loop
		default:
			var com = new(command.Command)
			// Blocking call that handles one command at a time
			err := su.decoder.Decode(com)
			if err != nil {
				su.handleCommunicationError("receives command from clients", err)
				break loop
			}
			// handle Command
			su.handleCommand(com)
		}
	}
}

func (su *User) handleCommand(com *command.Command) {
	var res *result.Result
	switch com.Ctype {
	case command.Send:
		su.Send(com)
		return
	case command.Private:
		res = su.Private(com)
	case command.End:
		res = su.End(com)
	case command.Who:
		res = su.Who(com)
	case command.Exit:
		res = su.Exit(com)
	case command.Create:
		res = su.Create(com)
	}
	if err := su.encoder.Encode(res); err != nil {
		su.handleCommunicationError("sends results to the client", err)
	}
}

// signOutEOF removes the user from any room he resides and notifies everybody
// the incident
func (su *User) signOutEOF() {
	if u := su.getUser(); u != nil {
		su.public.Remove(u.Username())
		su.public.Broadcaster() <- message.NewConcrete(&color.Reset,
			fmt.Sprintf("%s disconnected.", u.String()))
		// Log the incident
		log.Printf("user %s@%s closed connection unexpectedly", u.Username(), su.Conn.RemoteAddr().String())
	} else {
		log.Printf("connection on %s closed unexpectedly", su.Conn.RemoteAddr().String())
	}
}

func (su *User) handleCommunicationError(logPrefix string, err error) {
	if err == io.EOF {
		su.signOutEOF()
		return
	}
	log.Printf("%s: %s", logPrefix, err)
}

// Send corresponds to a send message operation which broadcasts the message
// to the entire room
func (su *User) Send(com *command.Command) {
	if su.getUser() == nil {
		err := su.encoder.Encode(result.New(result.Failure, "you didn't pick a username. Pick one with @name"))
		if err != nil {
			su.handleCommunicationError("send message", err)
			return
		}
		return
	}
	su.broadcastMessageChan <- su.Message(com.Args[0])
}

func (su *User) getUser() user.User {
	getter := userGetter{}
	getter.Add(1)
	su.getUserRequest <- &getter
	getter.Wait()
	return getter.User
}

// Private corresponds to the private command
func (su *User) Private(com *command.Command) *result.Result {
	if su.getUser() == nil {
		return result.New(result.Failure, "you didn't pick a username. Pick one with @name")
	}
	if len(com.Args) == 0 {
		return result.New(result.Failure, "private session not created: you didn't provide any names")
	} else if len(com.Args) == 1 && com.Args[0] == su.Username() {
		return result.New(result.Failure, "private session not created: please avoid adding yourself")
	}
	_, pris := su.public.WhoIsOnline()
	for _, pri := range pris {
		if pri == su {
			return result.New(result.Failure, "You cannot create a private session if you are in one")
		}
	}
	pri := private.New(su)
	su.public.PrivateAdder() <- pri
	su.public.ToPrivate(su, su.Username())
	for _, arg := range com.Args {
		if arg == su.Username() {
			su.Error() <- errors.New("you don't have to explicitly add yourself to the private session")
			continue
		}
		su.public.ToPrivate(su, arg)
	}
	return result.New(result.Success, "Private session created.")
}

// End ends the users' private session with some users
func (su *User) End(com *command.Command) *result.Result {
	if su.getUser() == nil {
		return result.New(result.Failure, "you didn't pick a username. Pick one with @name")
	}
	// no argument provided then remove the entire room
	if len(com.Args) == 0 {
		su.public.PrivateRemove(su)
	} else if len(com.Args) == 1 && com.Args[0] == su.Username() {
		su.Error() <- errors.New("user not removed: are you try'ng to remove yallself?")
	} else {
		for _, arg := range com.Args {
			su.public.ToPublic(su, arg)
		}
	}
	return result.New(result.Success, "")
}

// Exit closes the private room created by  this user or remove the user from
// the public
func (su *User) Exit(com *command.Command) *result.Result {
	// remove the user from public room
	if su.getUser() != nil {
		// remove have to wait until the remove operation is conducted
		// as this user might want to hear extra messages
		su.public.Remove(su.Username())
		su.public.Broadcaster() <- message.NewConcrete(&color.Reset, fmt.Sprintf("%s left.", su.Username()))
	} else {
		log.Printf("End connection with %s", su.RemoteAddr().String())
	}
	close(su.done)
	return result.New(result.Exit, "Signed out")
}

// Who prints all private users
func (su *User) Who(com *command.Command) *result.Result {
	pub, pri := su.public.WhoIsOnline()
	res := fmt.Sprintf("\n%sPublic:\n", color.Reset.String())
	if len(pub) == 0 {
		res += "\t...well...it's kinda empty now\n"
	} else {
		for _, user := range pub {
			res += fmt.Sprintf("\t%s\n", user.String())
		}
	}
	res += fmt.Sprint("Private:\n")
	if len(pri) == 0 {
		res += "\t...no one is talking behind your back.\n"
	} else {
		for _, user := range pri {
			res += fmt.Sprintf("\t%s\n", user.String())
		}
	}

	return result.New(result.Success, res)
}

// Create creates an user
func (su *User) Create(com *command.Command) *result.Result {
	if su.getUser() != nil {
		return result.New(result.Failure, "You already had a name")
	}
	if len(com.Args) == 0 || len(com.Args) > 1 {
		su.errChan <- errors.New("create a name: username is not provided or more than enough")
		return result.New(result.Success, "")
	} else if len(com.Args[0]) > 100 {
		su.errChan <- errors.New("create a name: username is too long ")
		return result.New(result.Success, "")
	}
	uc := userCreator{
		username: com.Args[0],
	}
	uc.Add(1)
	su.createRequest <- &uc
	uc.Wait()
	if uc.err != nil {
		return result.New(result.Failure, uc.err.Error())
	}
	// Add the user to the public room
	su.public.Adder() <- su
	return result.New(result.Success, "Name registering...")
}
