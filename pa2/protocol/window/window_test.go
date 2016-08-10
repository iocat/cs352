package window

import (
	"math"
	"testing"

	"github.com/iocat/rutgers-cs352/pa2/protocol/datagram/header"
)

func TestDistance(t *testing.T) {
	tests := []struct {
		a   header.Header
		b   header.Header
		res int
	}{
		{
			a: header.Header{
				Flag:     header.RED,
				Sequence: math.MaxUint32,
			},
			b: header.Header{
				Flag:     header.BLUE,
				Sequence: 1,
			},
			res: 2,
		},
		{
			a: header.Header{
				Flag:     header.RED,
				Sequence: 0,
			},
			b: header.Header{
				Flag:     header.RED,
				Sequence: 1,
			},
			res: 1,
		}, {
			a: header.Header{
				Flag:     header.BLUE,
				Sequence: 0,
			},
			b: header.Header{
				Flag:     header.BLUE,
				Sequence: math.MaxUint32,
			},
			res: math.MaxUint32,
		},
		{
			a: header.Header{
				Flag:     header.BLUE,
				Sequence: math.MaxUint32 - 2,
			},
			b: header.Header{
				Flag:     header.RED,
				Sequence: 0,
			},
			res: 3,
		},
	}
	for _, c := range tests {
		if d := distance(c.a, c.b); d != c.res {
			t.Errorf("distance(%#v,%#v) = %d, expect %d", c.a, c.b, d, c.res)
		}
	}
}
