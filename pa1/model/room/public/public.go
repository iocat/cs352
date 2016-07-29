package public

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/iocat/rutgers-cs352/pa1/model/color"
	"github.com/iocat/rutgers-cs352/pa1/model/message"
	room "github.com/iocat/rutgers-cs352/pa1/model/room/interf"
	"github.com/iocat/rutgers-cs352/pa1/model/room/private"
)

var (
	colorGreen  = color.New(color.DisplayStandout, color.FgGreen, color.BgBlack)
	brightGreen = color.New(color.DisplayBold, color.FgGreen, color.BgBlack)
	colorRed    = color.New(color.DisplayStandout, color.FgRed, color.BgBlack)
	brightRed   = color.New(color.DisplayBold, color.FgRed, color.BgBlack)
)

var (
	ErrDuplicateUsername = errors.New("add user: username existed")
)

type IRoom interface {
	Adder() chan<- room.User
	Remover() <-chan *private.UserRemoveOperation
	Remove(string)
	Count() int
	WhosThere() []room.User

	// Done lets the public room knows:
	// + when the private room is closed
	// + how to close the private room
	Done() chan struct{}
	Release()
	room.Broadcaster
}

type RoomWithOwner interface {
	IRoom
	Owner() room.User
}

// Room represents a public room and control access to the privates
type Room struct {
	count int
	limit int

	messageQueue []message.Message

	close chan struct{}
	// 2 channels that mask the underlying adder and remover channel
	// to control the number of users in the room
	userAdder    chan room.User
	userRemover  chan room.User
	broadcaster  chan message.Message
	userToRemove chan *publicRemoveOperation

	toPrivate chan *MigrationOperation
	toPublic  chan *MigrationOperation

	whoChan chan *whoOperation

	// errorChan sends error when the public channel encounter such
	adder    chan RoomWithOwner
	remover  chan RoomWithOwner
	counter  chan struct{}
	toRemove chan room.User

	privates map[string]RoomWithOwner
	users    map[string]room.User
}

type publicRemoveOperation struct {
	sync.WaitGroup
	username string
}

type whoOperation struct {
	sync.WaitGroup
	whoOnline struct {
		public  []room.User
		private []room.User
	}
}

func New(limit int) *Room {
	r := &Room{
		privates: make(map[string]RoomWithOwner),
		adder:    make(chan RoomWithOwner),
		remover:  make(chan RoomWithOwner),
		toRemove: make(chan room.User),

		users:        make(map[string]room.User),
		userAdder:    make(chan room.User),
		userRemover:  make(chan room.User),
		userToRemove: make(chan *publicRemoveOperation),
		toPublic:     make(chan *MigrationOperation),
		toPrivate:    make(chan *MigrationOperation),

		close: make(chan struct{}),

		counter: make(chan struct{}),
		limit:   limit,

		broadcaster: make(chan message.Message),

		whoChan: make(chan *whoOperation),
	}
	go r.listen()
	return r
}

func (r *Room) autoRoomMigration(priv RoomWithOwner) {
loop:
	for {
		select {
		// Private room close
		case <-priv.Done():
			break loop
			// User moves from private room back to public
		case op := <-priv.Remover():
			/*	go func(username string, br room.Broadcaster) {
				// Notify everyone in the private room this user had left
				br.Broadcaster() <- createNotification(
					fmt.Sprintf("%s left", username))
			}(usr.String(), priv)*/
			r.users[op.User.Username()] = op.User
			op.User.SetBroadcaster(r)
			op.Done()
		}
	}
}

func (r *Room) activeUsers() ([]room.User, []room.User) {
	pub := []room.User{}
	pri := []room.User{}
	for _, usr := range r.users {
		pub = append(pub, usr)
	}
	for _, private := range r.privates {
		pri = append(pri, private.WhosThere()...)
	}

	return pub, pri
}

func (r *Room) waitToAddUserToPrivate(host room.User, user string) {
	<-time.NewTimer(2 * time.Second).C
	r.ToPrivate(host, user)
}

func (r *Room) fromPublicToPrivate(host room.User, username string) {
	if private, ok := r.privates[host.Username()]; !ok {
		sendError(host.Error(), errors.New("move user to private: you do not host a private session"))
		return
	} else if user, ok := r.users[username]; !ok {
		// The specified user is not in the public room
		if r.isInPrivate(username) {
			go r.waitToAddUserToPrivate(host, username)
			sendError(host.Error(), fmt.Errorf("...wait until the user comes back to public mode"))
			return
		}
		sendError(host.Error(), fmt.Errorf("move user to private: user %s is not in the public room", username))
		return

	} else {
		user.SetBroadcaster(private)
		delete(r.users, username)
		private.Adder() <- user
		if user.Username() != host.Username() {
			user.Receive(
				createNotification(
					fmt.Sprintf("You are added to a private session hosted by %s", host.String())))
		}
	}
}

func (r *Room) fromPrivateToPublic(host room.User, username string) {
	var (
		ok      bool
		private RoomWithOwner
	)
	if private, ok = r.privates[host.Username()]; !ok {
		sendError(host.Error(), errors.New("move user to public: you do not host a private session"))
		return
	} else if !r.isInPrivate(username) {
		sendError(host.Error(), fmt.Errorf("%s is not in a private session", username))
		return
	}
	private.Broadcaster() <- createNotification(fmt.Sprintf("%s is removed from the private session.", username))
	// remove a user hence this user automatically goes back to public
	private.Remove(username)
}

func (r *Room) addPrivate(newR RoomWithOwner) {
	if _, ok := r.privates[newR.Owner().Username()]; ok {
		sendError(newR.Owner().Error(), fmt.Errorf("no private session created: %s already owned a private session", newR.Owner().Username()))
		return
	}
	// Check every single room to see whether this user is in private session or not
	r.privates[newR.Owner().Username()] = newR
	go r.autoRoomMigration(newR)

}

func (r *Room) removePrivate(host string) error {
	// if this user is hosting a private session
	if found, ok := r.privates[host]; ok {
		found.Broadcaster() <- createNotification("Private session closed. Releasing")
		found.Release()
		delete(r.privates, host)
	} else {
		return errors.New("you are not hosting any private session")
	}
	return nil
}

func (r *Room) isInPrivate(username string) bool {
	for _, priv := range r.privates {
		names := priv.WhosThere()
		for _, name := range names {
			if name.Username() == username {
				return true
			}
		}
	}
	return false
}

func (r *Room) addUser(user room.User) {
	r.count = len(r.users)
	for _, private := range r.privates {
		r.count += private.Count()
	}
	if r.count == r.limit {
		sendError(user.Error(), errors.New("add user: number of user exceeded limit"))
		return
	}
	// Identify duplicated names
	if _, ok := r.users[user.Username()]; ok {
		sendError(user.Error(), ErrDuplicateUsername)
		return
	}
	if r.isInPrivate(user.Username()) {
		sendError(user.Error(), ErrDuplicateUsername)
		return
	}
	r.users[user.Username()] = user
	// The user now send message to this room
	user.SetBroadcaster(r)
	// announce a user is online
	r.broadcast(message.NewConcrete(
		&color.Reset, fmt.Sprintf("%s is online", user.String())))
	r.sendLoggedMessages(user)
}

func (r *Room) sendLoggedMessages(user room.User) {
	for _, mes := range r.messageQueue {
		user.Receive(mes)
	}
}

func (r *Room) removeUser(username string) {
	// If the user is not in the public room
	// then ATTEMPT to get rid of him in every public room
	if _, ok := r.users[username]; !ok {
		// If user hosts a private room
		if private, ok := r.privates[username]; ok {
			// Host left send message
			private.Broadcaster() <- createNotification(
				fmt.Sprintf("Host %s left. Closing private session", username))
			r.removePrivate(username)
		} else {
			// If not, go to every single private room and
			// attempt to remove this user
			for _, private := range r.privates {
				private.Remove(username)
			}
		}
	}
	// Then check again, and delete this one from the public room
	if userToDelete, ok := r.users[username]; ok {
		delete(r.users, username)
		r.userRemover <- userToDelete
	}
}

func (r *Room) broadcast(mes message.Message) {
	for _, user := range r.users {
		user.Receive(mes)
	}
}

// listen synchronizes every operations that access shared states
func (r *Room) listen() {
loop:
	for {
		select {
		// Who finds out who is in the rooms public and private
		case wo := <-r.whoChan:
			wo.whoOnline.public, wo.whoOnline.private = r.activeUsers()
			wo.WaitGroup.Done()
		// sends user to private room
		case op := <-r.toPrivate:
			r.fromPublicToPrivate(op.host, op.username)
		// remove user from private room
		case op := <-r.toPublic:
			r.fromPrivateToPublic(op.host, op.username)
		// add new private room
		case newR := <-r.adder:
			r.addPrivate(newR)
		// remove a private room
		case host := <-r.toRemove:
			if err := r.removePrivate(host.Username()); err != nil {
				sendError(host.Error(), err)
			}
		// add new user to the public room
		case user := <-r.userAdder:
			r.addUser(user)
		// remove user from the public room and private if any
		case op := <-r.userToRemove:
			r.removeUser(op.username)
			op.Done()
		// broadcast a message
		case mes := <-r.broadcaster:
			r.log(mes)
			r.broadcast(mes)
		// room closed stop doing everything
		case <-r.close:
			break loop
		}
	}
}

func (r *Room) log(mes message.Message) {
	r.messageQueue = append(r.messageQueue, mes)
}

func createNotification(mes string) message.Message {
	return message.NewConcrete(
		&color.Reset,
		fmt.Sprintf(
			"%s%s%s%s",
			color.Green.String(), "SERVER: ", mes, color.Reset.String()))
}

func closePrivate(r RoomWithOwner) {
	r.Done() <- struct{}{}
}

func sendError(errChan chan<- error, err error) {
	// non-blocking error receiving
	go func() {
		errChan <- err
	}()
}
