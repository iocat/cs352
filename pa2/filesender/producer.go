package filesender

import (
	"io"
	"os"
)

// PacketProducer produces payload
type PacketProducer interface {
	Produce() ([]byte, error)
}

type filePacketProducer struct {
	file *os.File
	max  int
}

func newFileProducer(file *os.File, max int) PacketProducer {
	return filePacketProducer{
		file: file,
		max:  max,
	}
}

// Produce produces the next file payload
func (fpp filePacketProducer) Produce() ([]byte, error) {
	var next = make([]byte, fpp.max)
	n, err := fpp.file.Read(next)
	if err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}
	return next[:n], nil
}
