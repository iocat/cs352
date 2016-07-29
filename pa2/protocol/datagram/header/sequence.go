package header

import "encoding/binary"

// Sequence represents the sequence number of the packet
type Sequence uint32

// Bytes get the byte representation of the sequence number
func (sequence *Sequence) Bytes() []byte {
	var res = make([]byte, 4)
	binary.BigEndian.PutUint32(res, uint32(*sequence))
	return res
}
