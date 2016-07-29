package color

import (
	"fmt"
	"math/rand"
	"time"
)

const (
	DisplayNormal = iota
	DisplayBold
	DisplayFaint
	DisplayStandout
	DisplayUnderline
	DisplayBlink
	DisplayInverted
	DisplayHidden
)

const (
	DisplayNormal2 = 22 + iota
	DisplayNoStandout
	DisplayNoUnderline
	DisplayNoBlink
	DisplayNoReverse
)

// A set of foreground and background color
const (
	FgBlack = 30 + iota // start with 30
	FgRed
	FgGreen
	FgYellow
	FgBlue
	FgMagenta
	FgCyan
	FgWhite // to 37
	_       // Seriously DON'T remove this line
	FgDefault
	BgBlack // start with 40
	BgRed
	BgGreen
	BgYellow
	BgBlue
	BgMagenta
	BgCyan
	BgWhite // to 47
	_       // Again, DO NOT remove this line either
	BgDefault
)

var (
	Green       = New(DisplayStandout, FgGreen, BgBlack)
	BrightGreen = New(DisplayBold, FgGreen, BgBlack)
	Red         = New(DisplayStandout, FgRed, BgBlack)
	BrightRed   = New(DisplayBold, FgRed, BgBlack)
	Blue        = New(DisplayStandout, FgBlue, BgBlack)
	BrightBlue  = New(DisplayBold, FgBlue, BgBlack)
)

// Color of the message
type Color struct {
	display int
	fg      int
	bg      int
}

// Display returns the display behavior of color
func (color *Color) Display() int {
	return color.display
}

// Fg returns the foreground color
func (color *Color) Fg() int {
	return color.fg
}

// Bg returns the background color
func (color *Color) Bg() int {
	return color.bg
}

var (
	// Reset is a convenient way to reset the color
	Reset = Color{
		display: DisplayNormal,
	}
)

// New creates a new color
func New(display, fg, bg int) *Color {
	return &Color{
		display: display,
		fg:      fg,
		bg:      bg,
	}
}

func init() {
	// Seed the random function
	rand.Seed(time.Now().Unix())
}

// Randomize randomizes a valid color and returns it
func Randomize() *Color {
	randomize := func() *Color {
		return &Color{
			display: DisplayNormal + rand.Intn(DisplayStandout-DisplayNormal+1),
			fg:      FgBlack + rand.Intn(FgWhite-FgBlack+1),
			bg:      BgBlack, // + (rand.Int() % (BgWhite - BgBlack + 1)),
		}
	}
	color := randomize()
	for ; color.fgMatchesBg(); color = randomize() {
		continue
	}
	return color
}

// Match is used to check if the two colors are of a same color
func (color *Color) Match(other *Color) bool {
	return *color == *other
}

func (color *Color) fgMatchesBg() bool {
	return color.fg == color.bg-10
}

// Text representation of the color
func (color *Color) String() string {
	return fmt.Sprintf("\x1b[%d;%d;%dm", color.display, color.fg, color.bg)
}
