package filesender

import (
	"net"
)

// Addr represents the unique string form of an address
// Used as a key to query the actual address
type Addr string

func getAddr(addr *net.UDPAddr) Addr {
	return Addr(addr.String())
}
