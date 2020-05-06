package log4go

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Logger objects.
type Logger struct {
	name     string
	level    Level
	handlers []Handler
	parent   *Logger
	children []*Logger

	staged []*Record
}

var errNoFormatter = errors.New("handler has no formatter")

func newLogger(parent *Logger, name string, lvl Level, handlers ...Handler) *Logger {
	// use: sync.Pool ?
	log := &Logger{
		name:  name,
		level: lvl,
	}
	if parent != nil {
		log.parent = parent
		if parent.children == nil {
			parent.children = make([]*Logger, 0, 5)
		}
		parent.children = append(parent.children, log)
	}

	if len(handlers) > 0 {
		log.handlers = handlers
	}

	return log
}

// GetLogger returns a sub-logger (inherits traits from parent).
func (l *Logger) GetLogger(subName string) *Logger {
	// get/create a sub-logger

	loggerName := l.name
	if len(loggerName) > 0 {
		loggerName += "/"
	}
	loggerName += subName

	loggersLock.Lock()

	logger, exists := loggers[loggerName]
	if !exists {
		// create sub-logger
		logger = newLogger(l, loggerName, DEBUG)

		loggers[loggerName] = logger
	}

	loggersLock.Unlock()

	return logger
}

// SetLevel sets the logging level of the logger.
func (l *Logger) SetLevel(lvl Level) {
	if lvl == NOTSET {
		lvl = DEBUG
	}
	l.level = lvl
}

// Level returns the logger's (effective) level.
func (l *Logger) Level() Level {
	// as long as level is not set, ascend the ancestors
	for l.level == NOTSET {
		if l.parent != nil {
			l = l.parent
		} else { // no parent, use this logger's level
			break
		}
	}
	return l.level
}

// AddHandler adds a log record handler.
func (l *Logger) AddHandler(handler Handler) error {
	if handler.Formatter() == nil {
		return errNoFormatter
	}

	l.handlers = append(l.handlers, handler)
	return nil
}

// ReplaceHandlers replaces all added handler with a new handler.
func (l *Logger) ReplaceHandlers(handler Handler) {
	l.RemoveHandlers()
	l.AddHandler(handler)
}

// RemoveHandlers removes all handlers from the Logger.
func (l *Logger) RemoveHandlers() {
	l.handlers = []Handler{}
}

// Handlers returns all handlers used by this logger (i.e. this and all its parents' handlers).
func (l *Logger) Handlers() []Handler {
	handlers := make([]Handler, 0, 10)
	logger := l
	for logger != nil {
		if logger.handlers != nil && len(logger.handlers) > 0 {
			handlers = append(handlers, logger.handlers...)
		}
		logger = logger.parent
	}
	return handlers
}

// Log submits a Log message using specific level and message.
func (l *Logger) log(lvl Level, stage bool, message string, args ...interface{}) {
	if lvl < l.Level() || lvl == NOTSET {
		return
	}

	var record *Record

	// traverse up this logger's ancestors, calling all handlers along the way
	logger := l
	for logger != nil {
		if len(logger.handlers) > 0 { // we need handlers!
			if record == nil {
				record = recordPool.Get().(*Record)

				record.Time = time.Now()
				record.Name = l.name
				record.Level = lvl
				record.Message = fmt.Sprintf(message, args...)
			}

			if stage {
				if l.staged == nil {
					l.staged = make([]*Record, 0, 10)
				}
				l.staged = append(l.staged, record)
			} else {
				// invoke all handlers
				for _, handler := range logger.handlers {
					handler.Handle(record)
				}
			}
		}
		logger = logger.parent
	}

	if record != nil {
		recordPool.Put(record)
	}
}

func (l *Logger) flushStaged() {
	for _, r := range l.staged {
		for _, h := range l.handlers {
			h.Handle(r)
		}
	}
	l.staged = l.staged[:0]
}

// CrashOpts controls how Crash operates.
type CrashOpts struct {
	// BuildPath strips this prefix from all source file references in the stack trace.
	BuildPath string
	// ExitCode makes os.Exit(ExitCode), if set.
	ExitCode int
	// PlainStack instructs Crash to print the stack without path stripping or log formatting
	PlainStack bool
}

// Crash is similar to Fatal but also prints a stack trace.
func (l *Logger) Crash(err interface{}, stack []byte, opts ...CrashOpts) {
	// stack will always contain "useless" levels, e.g.:
	// runtime/debug.Stack(0xc4200ed7f0, 0x6aeee0, 0xc420101120)
	//    (location of call to debug.Stack())
	// main.main.func1(0xc420016f00)
	//    (location of deferred function that called recover())
	// panic(0x6aeee0, 0xc420101120)
	//    (location of call of panic())

	if len(l.staged) > 0 {
		l.flushStaged()
	}

	if len(opts) == 0 {
		opts = append(opts, CrashOpts{})
	}

	buildPath := opts[0].BuildPath
	exitCode := opts[0].ExitCode
	plainStack := opts[0].PlainStack

	lines := make([]string, 0, 20)
	skipped := 0

	// skip until we find "panic("
	reader := strings.NewReader(string(stack))
	for scanner := bufio.NewScanner(reader); scanner.Scan(); {
		line := scanner.Text()

		if plainStack {
			lines = append(lines, line)

		} else if skipped > 0 || strings.HasPrefix(line, "panic(") {
			skipped++

			if skipped >= 3 {
				if len(opts[0].BuildPath) > 0 && strings.HasPrefix(line, "\t"+buildPath) {
					line = "   " + line[2+len(buildPath):]
				} else if !strings.HasPrefix(line, "\t") {
					if parts := strings.SplitN(line, "/", -1); len(parts) > 1 {
						line = parts[len(parts)-1]
					}
				}

				lines = append(lines, line)
			}
		}
	}

	if plainStack {
		l.Error("CRASH: %v\n%s", err, strings.Join(lines, "\n"))

	} else {
		l.Error("CRASH: %v\n   %s", err, strings.Join(lines, "\n   "))

		//for _, line := range lines {
		//	l.Error(line)
		//}
	}

	if exitCode != 0 {
		Shutdown()
		os.Exit(exitCode)
	}
}

// ------------------------------------------------

// Fatal logs message with FATAL level (also does os.Exit(1)), after flushing staged messages.
func (l *Logger) Fatal(message string, args ...interface{}) {
	if len(l.staged) > 0 {
		l.flushStaged()
	}

	l.log(FATAL, false, message, args...)

	Shutdown()
	os.Exit(1)
}

// Error logs message with ERROR level, after flushing staged messages.
func (l *Logger) Error(message string, args ...interface{}) {
	if len(l.staged) > 0 {
		l.flushStaged()
	}
	l.log(ERROR, false, message, args...)
}

// Warning logs message with WARNING level (clears staged messages).
func (l *Logger) Warning(message string, args ...interface{}) {
	l.staged = l.staged[:0]
	l.log(WARNING, false, message, args...)
}

// Info logs message with INFO level (clears staged messages).
func (l *Logger) Info(message string, args ...interface{}) {
	l.staged = l.staged[:0]
	l.log(INFO, false, message, args...)
}

// Debug logs message with DEBUG level (clears staged messages).
func (l *Logger) Debug(message string, args ...interface{}) {
	l.staged = l.staged[:0]
	l.log(DEBUG, false, message, args...)
}

// Log logs message with given level (clears staged messages).
func (l *Logger) Log(lvl Level, message string, args ...interface{}) {
	if lvl == NOTSET {
		return
	}
	l.staged = l.staged[:0]
	l.log(lvl, false, message, args...)
}

// ------------------------------------------------

// StageWarning stages a message with WARNING level, flushed by Error() or Fatal().
func (l *Logger) StageWarning(message string, args ...interface{}) {
	l.log(WARNING, true, message, args...)
}

// StageInfo stages a message with INFO level, flushed by Error() or Fatal().
func (l *Logger) StageInfo(message string, args ...interface{}) {
	l.log(INFO, true, message, args...)
}

// StageDebug stages a message with DEBUG level, flushed by Error() or Fatal().
func (l *Logger) StageDebug(message string, args ...interface{}) {
	l.log(DEBUG, true, message, args...)
}

// StageLog stages a message with given level, flushed by Error() or Fatal().
func (l *Logger) StageLog(lvl Level, message string, args ...interface{}) {
	l.log(lvl, true, message, args...)
}
