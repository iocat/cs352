package header

var (
	// ACK indicates an Acknowledged packet
	ACK = Flag(1)
	// NACK indicates an not_acknowledged packet
	NACK = Flag(2)
	// FILE indicates a file name packet
	FILE = Flag(4)
	// EOF indicates an End of the file
	EOF = Flag(8)
	// EXIT indicates the receiver to exit
	EXIT = Flag(16)
	// RED indicates this is a red packet
	RED = Flag(32)
	// BLUE indicates this is a blue packet
	BLUE = Flag(64)
)

// Flag represents an unsigned byte
type Flag uint8

// Byte returns the byte representation of the flag
func (flag Flag) byte() byte {
	return byte(flag)
}

// IsRED returns whether this flag is a red
func (flag Flag) IsRED() bool {
	return flag&RED != 0
}

// IsFILE checks whether this flag is a file name
func (flag Flag) IsFILE() bool {
	return flag&FILE != 0
}

// IsNACK checks whether this flag is a NACK
func (flag Flag) IsNACK() bool {
	return flag&NACK != 0
}

// IsBLUE returns whether this flag is a blue
func (flag Flag) IsBLUE() bool {
	return flag&BLUE != 0
}

// IsACK returns whether this flag is an ACK or not
func (flag Flag) IsACK() bool {
	return flag&ACK != 0
}

// IsEOF returns whether this flag is an EOF or not
func (flag Flag) IsEOF() bool {
	return flag&EOF != 0
}

// IsEXIT returns whether this flag is an Exit signal or not
func (flag Flag) IsEXIT() bool {
	return flag&EXIT != 0
}

// Is checks whether this flag contains the provided list of flags
func (flag Flag) Is(flags ...Flag) bool {
	for _, f := range flags {
		if flag&f == 0 {
			return false
		}
	}
	return true
}

// RED decorates the flag as red
func (flag *Flag) RED() *Flag {
	*flag |= RED
	return flag
}

// BLUE decorates the flag as blue
func (flag *Flag) BLUE() *Flag {
	*flag |= BLUE
	return flag
}

// ACK decorates the flag with an ACK
func (flag *Flag) ACK() *Flag {
	*flag |= ACK
	return flag
}

// NACK decorates the flag with a NACK
func (flag *Flag) NACK() *Flag {
	*flag |= NACK
	return flag
}

// FILE decorates the flag with a FILE
func (flag *Flag) FILE() *Flag {
	*flag |= FILE
	return flag
}

// EOF decorates the flag with an EOF flag
func (flag *Flag) EOF() *Flag {
	*flag |= EOF
	return flag
}

// EXIT decorates the flag with an Exit signal
func (flag *Flag) EXIT() *Flag {
	*flag |= EXIT
	return flag
}
