package color

import (
	"testing"
)

// TestRandomizeColor tests whether the color is in the correct range
// or not
func TestRandomizeColor(t *testing.T) {
	colorSet := make(map[int]bool)
	for i := 0; i < 10000; i++ {
		color := Randomize()
		if color.fg < FgBlack || color.fg > FgWhite {
			t.Fatalf("randomize: color out of the expected range,"+
				"received %#vNOO!", color)
		}
		colorSet[color.fg] = true
	}
	if len(colorSet) != FgWhite-FgBlack+1 {
		t.Fatalf("randomize: did not receive enough color.Either retest or reimplement")
	}
}
