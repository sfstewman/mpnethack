package chat

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

type SystemLog struct {
	LogFile *os.File

	LogWriter  io.WriteCloser
	LineReader io.Reader

	LineCallback func(line string)

	mu sync.Mutex
}

func NewSystemLog(logPath string, lineCallback func(line string)) (*SystemLog, error) {
	f, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	pipeReader, pipeWriter := io.Pipe()
	tee := io.TeeReader(pipeReader, f)

	l := &SystemLog{
		LogFile: f,

		LogWriter:  pipeWriter,
		LineReader: tee,

		LineCallback: lineCallback,
	}

	if l.LineReader != nil {
		go l.handleLines()
	}

	log.SetOutput(l.LogWriter)

	return l, nil
}

func (l *SystemLog) SetCallback(cb func(string)) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.LineCallback = cb
}

func (l *SystemLog) handleLines() {
	l.mu.Lock()
	r := l.LineReader
	l.mu.Unlock()

	if r == nil {
		return
	}

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		l.addLine(sc.Text())
	}

	if err := sc.Err(); err != nil {
		io.WriteString(os.Stderr, fmt.Sprintf("error with logging: %v\n", err))
	}
}

func (l *SystemLog) addLine(line string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.LineCallback != nil {
		l.LineCallback(line)
	}
}

func (l *SystemLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var errs []error

	if log.Writer() == l.LogWriter {
		log.SetOutput(os.Stderr)
	}

	if l.LogWriter != nil {
		if e := l.LogWriter.Close(); e != nil {
			errs = append(errs, e)
		}

		l.LogWriter = nil
	}

	if l.LogFile != nil {
		if e := l.LogFile.Close(); e != nil {
			errs = append(errs, e)
		}

		l.LogFile = nil
	}

	l.LineReader = nil

	if len(errs) == 0 {
		return nil
	} else if len(errs) == 1 {
		return fmt.Errorf("error closing system logs: %w", errs[0])
	} else {
		firstErr := errs[0]
		restErrs := errs[1:]

		restErrStr := ""
		for _, e := range restErrs {
			restErrStr = restErrStr + fmt.Sprintf("\n  also encountered error: %v", e)
		}

		return fmt.Errorf("error closing system logs: %w%s", firstErr, restErrStr)
	}
}

func (l *SystemLog) Name() string {
	if l.LogFile == nil {
		return "<none>"
	}

	return l.LogFile.Name()
}
