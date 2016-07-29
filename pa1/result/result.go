package result

import (
	"fmt"

	"github.com/iocat/rutgers-cs352/pa1/model/color"
)

const (
	// Success represents a successful command
	Success = iota
	// Message represents a message received
	// from the server
	Message
	// Failure represents a failure
	Failure
	// Exit asks the client to exits the program
	Exit
	// Created confirms the user was created
	Created
)

const (
	resultFormat = "%d %s"
)

// Result is a concrete implementation of parser interface
// Result implements connection.Parser interface
type Result struct {
	Rtype   int
	Message string
}

// New creates a new result object
func New(rtype int, message string) *Result {
	return &Result{
		Rtype:   rtype,
		Message: message,
	}
}

// Colorize returns a message that has color :))
func (res *Result) Colorize(col *color.Color) string {
	return fmt.Sprintf("%s%s%s", col.String(), res.Message, color.Reset.String())
}
