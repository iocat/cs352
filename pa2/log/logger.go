package log

import (
	"log"
	"os"
)

var (
	// Warning log
	Warning = log.New(os.Stdout, "\x1B[91mWARNING: \x1B[39m", 0)
	// Debug log
	Debug = log.New(os.Stdout, "\x1B[96mDEBUG:   \x1B[39m", 0)
	// Info log
	Info = log.New(os.Stdout, "\x1B[92mINFO:    \x1B[39m", 0)
)
