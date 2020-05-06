package log4go

import "time"

// Record is a log message container.
type Record struct {
	Time    time.Time
	Name    string
	Level   Level
	Message string
}
