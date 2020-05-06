package log4go

import "fmt"

// Level is a typed logging level.
type Level int

// Log levels.
const (
	// NOTSET log level (inherits from parent).
	NOTSET Level = iota
	// TRACE log level.
	TRACE = 1
	// DEBUG log level.
	DEBUG = 2
	// INFO log level.
	INFO = 3
	// WARNING log level.
	WARNING = 4
	// ERROR log level.
	ERROR = 5
	// FATAL log level - globally unrecoverable error (also does os.Exit(1)).
	FATAL = 6
)

var levelToName = map[Level]string{
	NOTSET:  "NOTSET",
	FATAL:   "FATAL",
	ERROR:   "ERROR",
	WARNING: "WARNING",
	INFO:    "INFO",
	DEBUG:   "DEBUG",
	TRACE:   "TRACE",
}

// LevelName returns the textual representation of the level.
func LevelName(l Level) string {
	name, exists := levelToName[l]
	if !exists {
		name = fmt.Sprintf("%d", l)
	}
	return name
}
