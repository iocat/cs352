package client

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/iocat/rutgers-cs352/pa1/command"
	"github.com/iocat/rutgers-cs352/pa1/model/color"
	"github.com/iocat/rutgers-cs352/pa1/result"
)

const (
	userPrompt     = ""
	nameCommand    = "@name"
	privateCommand = "@private"
	endCommand     = "@end"
	whoCommand     = "@who"
	exitCommand    = "@exit"
)

func deleteEmpty(ss []string) []string {
	res := []string{}
	for _, s := range ss {
		if len(s) != 0 {
			res = append(res, s)
		}
	}
	return res
}

var (
	colorGreen  = color.New(color.DisplayStandout, color.FgGreen, color.BgBlack)
	brightGreen = color.New(color.DisplayBold, color.FgGreen, color.BgBlack)
	colorRed    = color.New(color.DisplayStandout, color.FgRed, color.BgBlack)
	brightRed   = color.New(color.DisplayBold, color.FgRed, color.BgBlack)
)

func parseCommand(processed string) *command.Command {
	var com = &command.Command{}
	if strings.HasPrefix(processed, privateCommand) {
		com = command.New(command.Private,
			deleteEmpty(strings.Split(strings.TrimLeft(processed, privateCommand), " ")))
	} else if strings.HasPrefix(processed, endCommand) {
		com = command.New(command.End,
			deleteEmpty(strings.Split(strings.TrimLeft(processed, endCommand), " ")))
	} else if strings.HasPrefix(processed, whoCommand) {
		com = command.New(command.Who, nil)
	} else if strings.HasPrefix(processed, exitCommand) {
		com = command.New(command.Exit, nil)
	} else if strings.HasPrefix(processed, nameCommand) {
		com = command.New(command.Create,
			deleteEmpty(strings.Split(strings.TrimLeft(processed, nameCommand), " ")))
	} else {
		com = command.New(command.Send, []string{processed})
	}
	return com
}

// Client represents a client that actively communicates with the server
type Client struct {
	conn     net.Conn
	encoder  *gob.Encoder
	decoder  *gob.Decoder
	username string
	scanner  *bufio.Scanner
	wg       sync.WaitGroup
	// done channel notifies the client to stop listening on the socket and end
	// programs
	done chan struct{}
}

// listenForResults actively listen for incoming data from the socket
// which is a result from a command
func (c *Client) listenForResults() {

forloop:
	for {
		select {
		case <-c.done:
			// Close the stdin
			_ = os.Stdin.Close()
			break forloop
		default:
			var res = new(result.Result)
			if err := c.decoder.Decode(&res); err != nil {
				c.handleCommunicationError("listen for results", err)
				break forloop
			}
			c.handleResult(res)
		}
	}
	c.wg.Done()
}

func (c *Client) listenForInputs() {
	//Receive input
forloop:
	for {
		select {
		case <-c.done:
			break forloop
		default:
			if c.scanner.Scan() {
				// Get next line
				processed := strings.Trim(c.scanner.Text(), " ")
				if len(processed) == 0 {
					// skip this command
					fmt.Print(userPrompt)
					break
				}
				com := parseCommand(processed)
				if err := c.encoder.Encode(com); err != nil {
					c.handleCommunicationError("send command error", err)
					break forloop
				}
				fmt.Print(userPrompt)
			}
		}
	}
	c.wg.Done()
}

// handleResult interprets the result and conduct some operation
func (c *Client) handleResult(res *result.Result) {
	switch res.Rtype {
	case result.Success:
		if len(res.Message) > 0 {
			fmt.Printf("%sSERVER: %s\n%s", brightGreen.String(), res.Colorize(colorGreen), color.Reset.String())
		}
	case result.Message:
		fmt.Println(res.Message)
	case result.Failure:
		fmt.Printf("%sSERVER ERROR: %s\n%s", brightRed.String(), res.Colorize(colorRed), color.Reset.String())
	case result.Exit:
		if len(res.Message) > 0 {
			fmt.Printf("%sSERVER: %s\n%s", brightGreen.String(), res.Colorize(colorGreen), color.Reset.String())
		}
		close(c.done)
	}
}

func (c *Client) handleCommunicationError(logPrefix string, err error) {
	defer close(c.done)
	if err == io.EOF {
		// Cleaning things up and close the connection
		fmt.Println("Connection to server closed unexpectedly")
		return
	}
	log.Printf("%s: %s", logPrefix, err)
}

// New creates a new Client
func New(conn net.Conn, username string) (*Client, error) {

	encoder := gob.NewEncoder(conn)
	decoder := gob.NewDecoder(conn)
	if err := encoder.Encode(
		command.New(command.Create, []string{username})); err != nil {
		return nil, err
	}
	return &Client{
		conn:     conn,
		username: username,
		encoder:  encoder,
		decoder:  decoder,
		scanner:  bufio.NewScanner(os.Stdin),
		done:     make(chan struct{}),
	}, nil
}

// Start starts a client
// This method spawns 2 threads
// + 1 that actively listens to server message
// + 1 thread on another hand listens to user input and command
func (c *Client) Start() {
	defer c.conn.Close()
	c.wg.Add(2)
	go c.listenForResults()
	go c.listenForInputs()
	c.wg.Wait()
}
