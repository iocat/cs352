package private

import (
	"errors"
	"sync"

	"github.com/iocat/rutgers-cs352/pa1/model/message"
	room "github.com/iocat/rutgers-cs352/pa1/model/room/interf"
)

type Room struct {
	adder        chan room.User
	remover      chan *UserRemoveOperation
	releaser     chan *sync.WaitGroup
	broadcaster  chan message.Message
	owner        room.User
	guessWhoChan chan *whoOperation

	removerWg sync.WaitGroup
	toRemove  chan string
	countChan chan *countOperation

	close chan struct{}

	users map[string]room.User
}

type UserRemoveOperation struct {
	sync.WaitGroup
	room.User
}

type countOperation struct {
	sync.WaitGroup
	count int
}

type whoOperation struct {
	people []room.User
	sync.WaitGroup
}

// WhosThere finds out who is in the room
func (r *Room) WhosThere() []room.User {
	wo := whoOperation{}
	wo.WaitGroup.Add(1)
	r.guessWhoChan <- &wo
	wo.WaitGroup.Wait()
	return wo.people
}

// Release removes every users
func (r *Room) Release() {
	var wg sync.WaitGroup
	wg.Add(1)
	r.releaser <- &wg
	wg.Wait()
}

// Adder returns a adder channel that adds users
func (r *Room) Adder() chan<- room.User {
	return r.adder
}

// Remover returns a remover channel that removes users
func (r *Room) Remover() <-chan *UserRemoveOperation {
	return r.remover
}

// Remove removes a user with username
// a call to remove have to wait until a user
// had completely removed from room
func (r *Room) Remove(username string) {
	r.removerWg.Add(1)
	r.toRemove <- username
	r.removerWg.Wait()
}

// Broadcaster asks to receive new message
func (r *Room) Broadcaster() chan<- message.Message {
	return r.broadcaster
}

// Owner returns the owner of the room
func (r *Room) Owner() room.User {
	return r.owner
}

// New returns a new room
func New(user room.User) *Room {
	r := &Room{
		owner:        user,
		guessWhoChan: make(chan *whoOperation),
		adder:        make(chan room.User),
		toRemove:     make(chan string),
		broadcaster:  make(chan message.Message),
		close:        make(chan struct{}),
		remover:      make(chan *UserRemoveOperation),
		users:        make(map[string]room.User),
		releaser:     make(chan *sync.WaitGroup),
		countChan:    make(chan *countOperation),
	}
	go r.listen()
	return r
}

// Count counts the number of user in the room
func (r *Room) Count() int {
	op := countOperation{}
	op.WaitGroup.Add(1)
	r.countChan <- &op
	op.Wait()
	return op.count
}

func (r *Room) removeUser(user room.User) {
	op := UserRemoveOperation{
		User: user,
	}
	op.Add(1)
	r.remover <- &op
	op.Wait()
}

// listen listens to incoming request and sends user out of the room
func (r *Room) listen() {
	defer close(r.adder)
	defer close(r.toRemove)
	defer close(r.broadcaster)
loop:
	for {
		select {
		case op := <-r.countChan:
			op.count = len(r.users)
			op.WaitGroup.Done()
		case op := <-r.guessWhoChan:
			for _, user := range r.users {
				op.people = append(op.people, user)
			}
			op.Done()
		case user := <-r.adder:
			if _, ok := r.users[user.Username()]; ok {
				sendError(user.Error(), errors.New("add user: username existed"))
			} else {
				r.users[user.Username()] = user
			}
		case username := <-r.toRemove:
			var (
				user room.User
				ok   bool
			)
			if user, ok = r.users[username]; ok {
				delete(r.users, username)
				r.removeUser(user)
			}
			r.removerWg.Done()
		case mes := <-r.broadcaster:
			for _, usr := range r.users {
				usr.Receive(mes)
			}
		case wg := <-r.releaser:
			for username, user := range r.users {
				delete(r.users, username)
				r.removeUser(user)
			}
			wg.Done()
		case <-r.close:
			// Release all users
			break loop
		}
	}
}

func sendError(errChan chan<- error, err error) {
	// non-blocking error receiving
	go func() {
		errChan <- err
	}()
}

// Done signifies the room  when it needs to be closed and removes everyone
func (r *Room) Done() chan struct{} {
	return r.close
}
