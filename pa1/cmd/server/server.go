package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/iocat/rutgers-cs352/pa1/server"
)

func parseArgs(args []string) (programName, port string, err error) {
	programName = args[0]
	if len(args) < 2 {
		err = errors.New("not enough argument: need a port number")
		return
	}
	port = args[1]
	return
}

func main() {
	program, port, err := parseArgs(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", program, err)
		return
	}
	var serv = server.New("tcp", fmt.Sprintf("localhost:%s", port))
	if err := serv.Start(); err != nil {
		log.Println(err)
	}
}
