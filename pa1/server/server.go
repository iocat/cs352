package server

// The way to use
// server.New() -> create a new server
// err := server.Start()... -> starts the server and listen to incoming connection
// err := server.Wait()...

import (
	"fmt"
	"log"
	"net"

	"github.com/iocat/rutgers-cs352/pa1/model/room/public"
	seruser "github.com/iocat/rutgers-cs352/pa1/server/user"
)

const chatRoomLimit = 20

// Chat represents a chat server that implements the Server
// interface. A chat server could be run simultaneously with a
// non-locking calls to Start
// A chat server also have a log
type Chat struct {
	address  string
	protocol string

	public  *public.Room
	errChan chan<- error
	net.Listener
}

func (chat *Chat) waitForRemovedUser() {
loop:
	for {
		select {
		case <-chat.public.Done():
			break loop
		case usr := <-chat.public.Remover():
			if usr, ok := usr.(*seruser.User); ok {
				log.Printf("User %s@%s left", usr.String(), usr.RemoteAddr().String())
			} else {
				log.Printf("User %s left", usr.String())
			}

		}
	}
}

// New creates a new server and makes it run on another goroutine
func New(protocol, address string) *Chat {
	chat := &Chat{
		protocol: protocol,
		address:  address,
		public:   public.New(chatRoomLimit),
	}
	return chat
}

func (chat *Chat) run() {
	for {
		if newConn, err := chat.Accept(); err != nil {
			// Error receiving an error, log it
			continue
		} else {
			log.Printf("Accepted connection from %s\n",
				newConn.RemoteAddr().String())
			// Receive a new connect request from a host
			go chat.handleClientConn(newConn)
		}
	}

}

func (chat *Chat) handleClientConn(conn net.Conn) {
	if err := chat.createNewUser(conn); err != nil {

	}
}

func (chat *Chat) createNewUser(conn net.Conn) error {
	if _, err := seruser.New(chat.public, conn); err != nil {
		return fmt.Errorf("create a new server user: %s", err)
	}
	return nil
}

// Start starts a server on another goroutine
func (chat *Chat) Start() error {
	defer chat.public.Close()
	var err error
	if chat.Listener, err = net.Listen(chat.protocol, chat.address); err != nil {
		return fmt.Errorf("establish connection: %s", err)
	}
	go chat.waitForRemovedUser()
	chat.run()
	return nil
}
