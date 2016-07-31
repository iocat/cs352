package header

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	// HeaderSizeInBytes is the size of the header = sizeof(Header)
	HeaderSizeInBytes = 5
)

// GoString returns the string representation of the object
func (header Header) GoString() string {
	var flag string
	var color string
	switch {
	case header.IsACK():
		flag = "ACK"
	case header.IsEOF():
		flag = "EOF"
	case header.IsEXIT():
		flag = "EXIT"
	case header.IsFILE():
		flag = "FILE"
	case header.IsNACK():
		flag = "NACK"
	default:
		flag = "none"
	}
	switch {
	case header.IsRED():
		color = "RED"
	case header.IsBLUE():
		color = "BLUE"
	default:
		color = "none"
	}
	return fmt.Sprintf("Header(%s,%s,%d)", flag, color, header.Sequence)
}

// Header represents a packet header
// that has a flag and a sequence number
type Header struct {
	Flag
	Sequence
}

// Bytes returns the byte representation of the header
// (5 bytes long)
func (header *Header) Bytes() []byte {
	var res = make([]byte, 5)
	res[0] = header.Flag.byte()
	copy(res[1:], header.Sequence.Bytes())
	return res
}

// PureHeader returns a header that has only the color associated with it
// and the sequence number
func (header *Header) PureHeader() Header {
	var pure Header
	pure = *header
	// set the flag to whether it is red or not
	pure.Flag &= (RED | BLUE)
	return pure
}

// Compare returns 0 if the header is equal the other
// 1 if bigger and -1 if smaller
// This comparision is based on the concept of RED-BLUE infinite
// RED-BLUE sequencing
func (header Header) Compare(other Header) int {
	var (
		this = header.PureHeader()
		that = other.PureHeader()
	)
	if this == that {
		return 0
	} else if (this.IsRED() && that.IsRED()) ||
		(this.IsBLUE() && that.IsBLUE()) {
		if this.Sequence > that.Sequence {
			return 1
		}
		return -1
	} else if this.IsRED() && that.IsBLUE() ||
		this.IsBLUE() && that.IsRED() {
		if this.Sequence > that.Sequence {
			return -1
		}
		return 1
	}
	panic("incorrect header provided")
}

// New creates a new Header using the provided flag and sequence number
func New(flag Flag, sequence Sequence) *Header {
	return &Header{
		Flag:     flag,
		Sequence: sequence,
	}
}

// Next returns the next header in the sequence
// the header is only provided with a color and the next sequence number
// No extra information is marked in the flag
func (header Header) Next() Header {
	if header.Sequence == math.MaxUint32 {
		if header.IsRED() {
			return Header{
				Flag:     BLUE,
				Sequence: 0,
			}
		}
		return Header{
			Flag:     RED,
			Sequence: 0,
		}

	}
	return Header{
		Flag:     header.Flag,
		Sequence: header.Sequence + 1,
	}
}

// NewFromBytes creates a new Header from the byte
// the size of the parameter must be 5 or bigger or a panic will occur
func NewFromBytes(header []byte) *Header {
	var (
		flag     = Flag(header[0])
		sequence = Sequence(binary.BigEndian.Uint32(header[1:]))
	)
	return &Header{
		Flag:     flag,
		Sequence: sequence,
	}
}
