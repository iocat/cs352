package log

import (
	"log"
	"os"
)

var (
	// Warning log
	Warning = log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime)
	// Debug log
	Debug = log.New(os.Stdout, "DEBUG:   ", log.Ldate|log.Ltime)
	// Info log
	Info = log.New(os.Stdout, "INFO:    ", log.Ldate|log.Ltime)
)
