package public

import (
	"github.com/iocat/rutgers-cs352/pa1/model/message"
	room "github.com/iocat/rutgers-cs352/pa1/model/room/interf"
)

// Remover returns a user channel that the client code waits
func (r *Room) Remover() <-chan room.User {
	return r.userRemover
}

// WhoIsOnline gets 2 lists :
// the ones that are in public room and the ones that are not
func (r *Room) WhoIsOnline() (public []room.User, private []room.User) {
	wo := whoOperation{}
	wo.WaitGroup.Add(1)
	// Send the operation then wait
	r.whoChan <- &wo
	wo.WaitGroup.Wait()
	return wo.whoOnline.public, wo.whoOnline.private
}

func (r *Room) PrivateAdder() chan<- RoomWithOwner {
	return r.adder
}

func (r *Room) PrivateRemove(host room.User) {
	r.toRemove <- host
}

type MigrationOperation struct {
	host     room.User
	username string
}

func (r *Room) ToPrivate(host room.User, username string) {
	op := MigrationOperation{
		host:     host,
		username: username,
	}
	r.toPrivate <- &op
}

func (r *Room) ToPublic(host room.User, username string) {
	op := MigrationOperation{
		host:     host,
		username: username,
	}
	r.toPublic <- &op
}

// Adder returns a masked channel adder
func (r *Room) Adder() chan<- room.User {
	return r.userAdder
}

// Remove removes a user by sending message to a masked channel
func (r *Room) Remove(username string) {
	op := publicRemoveOperation{
		username: username,
	}
	op.Add(1)
	r.userToRemove <- &op
	op.Wait()
}

// Broadcaster returns a channel message
func (r *Room) Broadcaster() chan<- message.Message {
	return r.broadcaster
}

// Close closes the room
func (r *Room) Close() {
	close(r.close)
}

// Done notifies the client code that the room is closed
func (r *Room) Done() <-chan struct{} {
	return r.close
}
