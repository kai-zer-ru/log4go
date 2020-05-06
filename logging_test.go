package log4go

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kaizer666/log4go/color"
)

func TestOne(t *testing.T) {
	var buf bytes.Buffer

	formatter, err := NewTemplateFormatter("{name} {level} {message}")
	if err != nil {
		t.Error(err)
	}
	formatter.EnableLevelColoring(false)
	formatter.EnablePatternColoring(false)
	BasicConfig(BasicConfigOpts{
		Level:  DEBUG,
		Writer: &buf,
		Format: formatter.GetFormat(),
	})
	log := GetLogger()

	for idx := 0; idx < 100; idx++ {
		log.Info("test message %d", idx)
	}

	Shutdown()

	foundLast := false
	scanner := bufio.NewScanner(&buf)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasSuffix(line, "test message 99") {
			foundLast = true
		}
	}

	if !foundLast {
		t.Errorf("last message not found (output len: %d)", buf.Len())
	}
}

func TestOneTwo(t *testing.T) {
	var fileName = "main.log"
	_ = os.Remove(fileName)

	formatter, err := NewTemplateFormatter("{name} {level} {message}")
	if err != nil {
		t.Error(err)
	}
	formatter.EnableLevelColoring(false)
	formatter.EnablePatternColoring(false)
	BasicConfig(BasicConfigOpts{
		Level:            DEBUG,
		FileName:         fileName,
		FileAppend:       true,
		Format:           formatter.GetFormat(),
		WriteStartHeader: true,
	})
	log := GetLogger()

	for idx := 0; idx < 100; idx++ {
		log.Info("test message %d", idx)
	}

	Shutdown()

	foundLast := false
	file, err := os.Open(fileName)
	if err != nil {
		t.Error(err)
	}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		t.Log(line)
		if strings.HasSuffix(line, "test message 99") {
			foundLast = true
		}
	}

	if !foundLast {
		s, _ := file.Stat()
		t.Errorf("last message not found (output len: %d)", s.Size())
	}
}

func TestOnlyChildLogger(t *testing.T) {

	GetLogger().RemoveHandlers() // no logging from root logger

	var buf bytes.Buffer
	fp := &buf
	//fp, _ := os.OpenFile("TestOnlyChildLogger.log", os.O_CREATE | os.O_TRUNC, 0664)
	handler, _ := NewStreamHandler(fp)
	fmt, err := NewTemplateFormatter("{name} {levelwsda} {message}")
	if err == nil {
		t.Error("no error in wrong format")
	}
	fmt, err = NewTemplateFormatter("{name} {level} {message}")
	if err != nil {
		t.Error(err)
	}
	err = fmt.SetFormat("{name} {level} {message}")
	if err != nil {
		t.Error(err)
	}
	err = fmt.SetFormat("ame} Asdzx]}' {le")
	if err == nil {
		t.Error("no error")
	}
	err = fmt.SetFormat("{name} {level} {message<12345}")
	if err != nil {
		t.Error(err)
	}
	fmt.EnableLevelColoring(true)
	fmt.SetPatternColoring(map[string]string{
		"brackets": color.Purple,
		"punct":    color.Blue,
		"quoted":   color.Red,
	}, []PatternColor{
		{"brackets", regexp.MustCompile(`([<>\]\(\)\{\}]|\[)`)}, // all kinds of brackets
		{"punct", regexp.MustCompile(`([-/\*\+\.,:])`)},
		{"quoted", regexp.MustCompile(`('[^']+'|"[^"]+")`)}, // quoted strings
	})
	fmt.SetLevelColoring(map[Level]string{
		FATAL:   color.RedBg + color.Bold,
		ERROR:   color.Red,
		WARNING: color.Yellow,
		INFO:    color.Normal,
		DEBUG:   color.RedBg,
	})
	fmt.EnablePatternColoring(true)

	handler.SetFormatter(fmt)
	log := GetLogger()
	log.AddHandler(handler)
	log.SetLevel(INFO) // otherwise it will inherit root's WARNING (the default)

	log.Info("test message 99")

	Shutdown()

	foundLast := false
	scanner := bufio.NewScanner(&buf)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "test message 99") {
			foundLast = true
		}
	}

	if !foundLast {
		t.Errorf("last message not found (output len: %d)", buf.Len())
	}
}

func TestLevelFilter(t *testing.T) {
	var buf = bytes.Buffer{}
	BasicConfig(BasicConfigOpts{
		Writer: &buf,
	})
	log := GetLogger()
	log.Info("this will never appear in the log")
	l := len(buf.Bytes())
	Shutdown()
	if l != 0 {
		t.Errorf("expected empty log, got %d bytes", l)
	}
}

func TestNoHandlers(t *testing.T) {
	var buf bytes.Buffer

	BasicConfig(BasicConfigOpts{
		Level:  DEBUG,
		Writer: &buf,
	})

	GetLogger().RemoveHandlers()

	log := GetLogger()

	log.Info("this will never appear in the log")

	Shutdown()

	if buf.Len() != 0 {
		t.Errorf("expected empty log, got %d bytes", buf.Len())
	}
}

func TestMulti(t *testing.T) {
	var buf bytes.Buffer

	BasicConfig(BasicConfigOpts{
		Level:     DEBUG,
		Writer:    &buf,
		WatchFile: true,
	})

	width := 100

	done := make(chan bool, width)

	for idx := 0; idx < width; idx++ {
		log := GetLogger(fmt.Sprintf("test%d", idx))

		go func(log *Logger) {
			for idx := 0; idx < width; idx++ {
				log.Info("test message %d", idx)
			}
			done <- true
		}(log)
	}

	for idx := 0; idx < width; idx++ {
		<-done
	}

	Shutdown()

	var foundLast int
	scanner := bufio.NewScanner(&buf)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "test message 99") {
			foundLast++
		}
	}

	if foundLast != width {
		t.Errorf("found %d last messages, expected %d", foundLast, width)
	}
}

func BenchmarkAllLogged(b *testing.B) {
	BasicConfig(BasicConfigOpts{
		FileName: "/dev/null",
	})

	log := GetLogger()

	startTime := time.Now()

	for idx := 0; idx < b.N; idx++ {
		log.Info("test message %d", idx)
	}

	duration := time.Now().Sub(startTime)
	Shutdown()

	printPerf(b.N, duration)
}

func BenchmarkNoneLogged(b *testing.B) {
	BasicConfig(BasicConfigOpts{
		Level:    WARNING, // thus all info-logs below will not be output
		FileName: "/dev/null",
	})

	log := GetLogger()

	startTime := time.Now()

	for idx := 0; idx < b.N; idx++ {
		log.Info("test message %d", idx)
	}

	Shutdown()
	duration := time.Now().Sub(startTime)
	printPerf(b.N, duration)
}

func BenchmarkMultiAllLogged(b *testing.B) {
	BasicConfig(BasicConfigOpts{
		Level:    DEBUG,
		FileName: "/dev/null",
	})

	startTime := time.Now()

	width := 100

	done := make(chan bool, width)

	for idx := 0; idx < width; idx++ {
		log := GetLogger(fmt.Sprintf("test%d", idx))

		go func(log *Logger) {
			for idx := 0; idx < b.N; idx++ {
				log.Info("test message %d", idx)
			}
			done <- true
		}(log)
	}

	for idx := 0; idx < width; idx++ {
		<-done
	}

	Shutdown()
	duration := time.Now().Sub(startTime)

	printPerf(width*b.N, duration)
}

func printPerf(n int, d time.Duration) {
	secs := d.Seconds()

	fmt.Fprintf(os.Stderr, "%d logs in %.3f ms -> %.0f logs/s  avg: %.3f µs/msg\n",
		n,
		secs*1e3,
		float64(n)/secs,
		secs*1e6/float64(n),
	)

}
