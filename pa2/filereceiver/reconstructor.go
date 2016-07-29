package filereceiver

import (
	"io"
	"os"

	"github.com/iocat/rutgers-cs352/pa2/log"
)

// reconstructFile is a blocking call that reconstruct a file based on
// the byte array stream.
// if the length of the received payload is 0, the function returns and
// the file is closed
func reconstructFile(file io.WriteCloser, payloads <-chan []byte) {
	defer file.Close()
	for payload := range payloads {
		if len(payload) == 0 {
			// The length of the payload is 0
			return
		}
		var (
			length int
			err    error
		)
		if length, err = file.Write(payload); err != nil {
			log.Warning.Printf("reconstruct file: write to file error: %s", err)
			return
		}
		go func(size int) {
			if file, ok := file.(*os.File); ok {
				log.Debug.Printf("reconstruct file %s: added %d bytes", file.Name(), size)
			}
		}(length)
	}
}
