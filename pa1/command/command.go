package command

const (
	// Send sends the message to the entire room
	Send = iota
	// Private creates a private chatroom
	Private
	// End ends the private session of a specified user
	End
	// Who gets people in the room
	Who
	// Exit creates an exit request
	Exit
	// Create creates a new user
	Create
)

const (
	commandFormat = "%d %v"
)

// Command represents a command that can parse a string and return the command
// Command is the common language that both the clients and the server
// understand
// Command implements connection.Parser interface
type Command struct {
	Args  []string
	Ctype int
}

// New creates a new Command
func New(ctype int, args []string) *Command {
	return &Command{
		Args:  args,
		Ctype: ctype,
	}
}
