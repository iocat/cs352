package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/iocat/rutgers-cs352/pa1/client"
)

var programName string

func parseArgs(args []string) (programName, address string, port int,
	username string, err error) {
	var (
		portString string
	)
	programName = args[0]
	if len(args) < 3 {
		err = errors.New("not enough argument: please provide enough arguments")
		return
	}
	address, portString = args[1], args[2]
	port, err = strconv.Atoi(portString)
	if err != nil {
		err = fmt.Errorf("invalid port number")
	}
	return
}

func connectThroughTCP(host string, port int) (net.Conn, error) {
	address := fmt.Sprintf("%s:%d", host, port)
	connection, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("get connection: %v", err)
	}
	fmt.Println("Successfully connected to the server.")
	return connection, err
}

func main() {
	var (
		programName string
		host        string
		port        int
		username    string
		conn        net.Conn
		err         error
	)
	// parse program parameters
	programName, host, port, username, err = parseArgs(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: parse error: %v\n", programName, err)
		os.Exit(1)
	}
	// Set up TCP connection and connect to the server at the same time
	conn, err = connectThroughTCP(host, port)
	for err != nil {
		fmt.Fprintf(os.Stderr, "%s: connect to server: %v\n", programName, err)
		fmt.Fprintf(os.Stderr, "Reconnecting...\n")
		<-time.NewTimer(3 * time.Second).C
		conn, err = connectThroughTCP(host, port)
	}

	cli, err := client.New(conn, username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create user on server: %s\n", err)
		os.Exit(1)
	}
	cli.Start()

}
