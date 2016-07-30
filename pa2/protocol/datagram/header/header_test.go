package header

import (
	"math"
	"testing"
)

// TestCompare tests the comparing between 2 headers
func TestCompare(t *testing.T) {
	cases := []struct {
		header1  Header
		header2  Header
		expected int
	}{
		{
			header1: Header{
				Flag:     RED,
				Sequence: 0,
			},
			header2: Header{
				Flag:     RED,
				Sequence: math.MaxUint32,
			},
			expected: -1,
		},
		{
			header1: Header{
				Flag:     BLUE,
				Sequence: 1000,
			},
			header2: Header{
				Flag:     BLUE,
				Sequence: 2,
			},
			expected: 1,
		},
		{
			header1: Header{
				Flag:     BLUE,
				Sequence: 120,
			},
			header2: Header{
				Flag:     RED,
				Sequence: 1,
			},
			expected: -1,
		},
		{
			header1: Header{
				Flag:     RED,
				Sequence: 2000,
			},
			header2: Header{
				Flag:     BLUE,
				Sequence: 10000,
			},
			expected: 1,
		},
	}
	for _, c := range cases {
		if res := c.header1.Compare(c.header2); c.expected != res {
			t.Errorf("compare %#v to %#v returns %d, expected %d", c.header1,
				c.header2, res, c.expected)
		} else {
			t.Logf("compare %#v to %#v: passed", c.header1, c.header2)
		}
	}
}
