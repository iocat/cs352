package header

const (
	flagACK = 1 << iota
	flagNACK
	flagFILE
	flagEOF
	flagEXIT
	flagRED
	flagBLUE
)

var (
	// ACK indicates an Acknowledged packet
	ACK = *new(Flag).ACK()
	// NACK indicates an not_acknowledged packet
	NACK = *new(Flag).NACK()
	// FILE indicates a file name packet
	FILE = *new(Flag).FILE()
	// EOF indicates an End of the file
	EOF = *new(Flag).EOF()
	// EXIT indicates the receiver to exit
	EXIT = *new(Flag).EXIT()
	// RED indicates this is a red packet
	RED = *new(Flag).RED()
	// BLUE indicates this is a blue packet
	BLUE = *new(Flag).BLUE()
)

// Flag represents an unsigned byte
type Flag uint8

// Byte returns the byte representation of the flag
func (flag *Flag) byte() byte {
	return byte(*flag)
}

// IsRED returns whether this flag is a red
func (flag *Flag) IsRED() bool {
	return *flag&flagRED != 0
}

// IsFILE checks whether this flag is a file name
func (flag *Flag) IsFILE() bool {
	return *flag&flagFILE != 0
}

// IsNACK checks whether this flag is a NACK
func (flag *Flag) IsNACK() bool {
	return *flag&flagNACK != 0
}

// IsBLUE returns whether this flag is a blue
func (flag *Flag) IsBLUE() bool {
	return *flag&flagBLUE == 0
}

// IsACK returns whether this flag is an ACK or not
func (flag *Flag) IsACK() bool {
	return *flag&flagACK != 0
}

// IsEOF returns whether this flag is an EOF or not
func (flag *Flag) IsEOF() bool {
	return *flag&flagEOF != 0
}

// IsEXIT returns whether this flag is an Exit signal or not
func (flag *Flag) IsEXIT() bool {
	return *flag&flagEXIT != 0
}

// Is checks whether this flag contains the provided list of flags
func (flag *Flag) Is(flags ...Flag) bool {
	for _, f := range flags {
		if *flag&f == 0 {
			return false
		}
	}
	return true
}

// RED decorates the flag as red
func (flag *Flag) RED() *Flag {
	*flag |= flagRED
	return flag
}

// BLUE decorates the flag as blue
func (flag *Flag) BLUE() *Flag {
	*flag |= flagBLUE
	return flag
}

// ACK decorates the flag with an ACK
func (flag *Flag) ACK() *Flag {
	*flag |= flagACK
	return flag
}

// NACK decorates the flag with a NACK
func (flag *Flag) NACK() *Flag {
	*flag |= flagNACK
	return flag
}

// FILE decorates the flag with a FILE
func (flag *Flag) FILE() *Flag {
	*flag |= flagFILE
	return flag
}

// EOF decorates the flag with an EOF flag
func (flag *Flag) EOF() *Flag {
	*flag |= flagEOF
	return flag
}

// EXIT decorates the flag with an Exit signal
func (flag *Flag) EXIT() *Flag {
	*flag |= flagEXIT
	return flag
}
