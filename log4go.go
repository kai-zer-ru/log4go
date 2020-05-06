// Package log4go provides a simple, tree-like logging facility.
package log4go

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// BasicConfigOpts is used to supply options to BasicConfig.
type BasicConfigOpts struct {
	FileName         string
	FileAppend       interface{}
	WatchFile        bool
	Writer           io.Writer
	Format           string
	Level            Level
	Handlers         []Handler
	WriteStartHeader bool
}

var rootLogger *Logger
var loggersLock = &sync.Mutex{}
var loggers map[string]*Logger

var recordPool sync.Pool

func init() {
	recordPool = sync.Pool{
		New: func() interface{} {
			return &Record{}
		},
	}

	loggers = make(map[string]*Logger)
}

// BasicConfig sets up a simple configuration of the logging system.
func BasicConfig(opts BasicConfigOpts) error {
	loggersLock.Lock()
	defer loggersLock.Unlock()

	// remove any/all created Logger, Handler and Formatter instances
	Shutdown()
	loggers = map[string]*Logger{}
	rootLogger = nil

	var err error

	if opts.Level == 0 {
		opts.Level = WARNING
	} else if opts.Level == NOTSET {
		opts.Level = DEBUG
	}
	if len(opts.Format) == 0 {
		opts.Format = "{time} {name<20} {level<8} {message}"
	}

	if len(opts.Handlers) == 0 {
		var defHandler Handler
		var err error

		if opts.WatchFile && opts.Writer != nil {
			opts.WatchFile = false
		}

		if opts.Writer != nil {
			defHandler, err = NewStreamHandler(opts.Writer)
		} else if len(opts.FileName) > 0 {
			append := opts.FileAppend == nil || opts.FileAppend.(bool)

			if opts.WatchFile {
				defHandler, err = NewWatchedFileHandler(opts.FileName, append, opts.WriteStartHeader)
			} else {
				defHandler, err = NewFileHandler(opts.FileName, append, opts.WriteStartHeader)
			}
		} else {
			defHandler, err = NewStreamHandler(os.Stderr)
		}
		if err != nil {
			return err
		}
		opts.Handlers = []Handler{defHandler}
	}

	// use a default formatter if the specified handler(s) has none
	var defFormatter Formatter
	for _, handler := range opts.Handlers {
		if handler.Formatter() == nil {
			if defFormatter == nil { // create a default formatter
				defFormatter, err = NewTemplateFormatter(opts.Format)
				if err != nil {
					return err
				}
			}
			handler.SetFormatter(defFormatter)
		}
	}

	rootLogger = createRootLogger(opts.Handlers...)
	rootLogger.SetLevel(opts.Level)

	return nil
}

// Shutdown shuts down all internals of log4go.
func Shutdown() {
	// close all commit channels

	// first collect all unique handlers
	uniqueHandlers := make(map[string]Handler, 10)
	collectHandlers(rootLogger, uniqueHandlers)
	allHandlers := make([]Handler, 0, len(uniqueHandlers))
	for _, h := range uniqueHandlers {
		allHandlers = append(allHandlers, h)
	}
	// then shut them all down
	shutdownHandlers(allHandlers)

	runtime.Gosched()
	runtime.GC()
	syscall.Sync()

	// TODO: wait for the commiters to finish (somehow)

	// nice synchronization there, Bob!
	time.Sleep(100 * time.Millisecond)
}

func collectHandlers(log *Logger, uniqueHandlers map[string]Handler) {
	if log == nil {
		return
	}
	if log.children != nil {
		for _, child := range log.children {
			collectHandlers(child, uniqueHandlers)
		}
	}

	if log.handlers != nil {
		for _, h := range log.handlers {
			// use the pointer address as the unique key
			hkey := fmt.Sprintf("%p", h)

			uniqueHandlers[hkey] = h // might already exists, but it'll be the same handler
		}
	}
}
func shutdownHandlers(allHandlers []Handler) {
	for _, h := range allHandlers {
		h.Shutdown()
	}
}

// GetLogger returns the root logger while GetLogger(name) calls GetLogger(name) on the root logger.
func GetLogger(name ...string) *Logger {
	if len(name) > 0 && !(len(name) == 1 && name[0] == "root") {
		return GetLogger().GetLogger(name[0])
	}

	// get/create the root logger
	loggersLock.Lock()
	defer loggersLock.Unlock()

	if rootLogger == nil {
		rootLogger = createRootLogger()
	}

	return rootLogger
}

func createRootLogger(handlers ...Handler) *Logger {
	//fmt.Println("creating root logger: %d handlers", len(handlers))

	if len(handlers) == 0 {
		handler, _ := NewStreamHandler(os.Stderr)
		formatter, _ := NewTemplateFormatter("{time} {name} {level} {message}")
		handler.SetFormatter(formatter)
		handlers = []Handler{handler}
	}

	//fmt.Printf("root logger, h = %p\n", handler)

	logger := newLogger(nil, "", WARNING, handlers...)

	return logger
}
