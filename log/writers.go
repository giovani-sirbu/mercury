package log

import (
	"io"
	"os"
	"sync"

	logs "github.com/sirupsen/logrus"
)

// errorWriter / infoWriter / warnWriter are the per-level multiwriters
// (stdout + file). Initialized lazily on first use via sync.Once so the
// open(2) syscall + file descriptor allocation happens once per process
// rather than on every log call. Previously each Error/Info/Warn invocation
// opened the file, wrote a line, then closed — on the hot path (action
// chain logs per tick, broker logs per message) this was thousands of
// open/close syscalls per second.
//
// If a file cannot be opened (read-only container FS, etc.) we fall back
// to stdout only. logrus is already safe for concurrent writes from
// multiple goroutines, so no extra mutex is needed here.
var (
	errorWriter   io.Writer
	infoWriter    io.Writer
	warnWriter    io.Writer
	errorWriterMu sync.Once
	infoWriterMu  sync.Once
	warnWriterMu  sync.Once
)

func getErrorWriter() io.Writer {
	errorWriterMu.Do(func() {
		errorWriter = openLogWriter("error.log")
	})
	return errorWriter
}

func getInfoWriter() io.Writer {
	infoWriterMu.Do(func() {
		infoWriter = openLogWriter("info.log")
	})
	return infoWriter
}

func getWarnWriter() io.Writer {
	warnWriterMu.Do(func() {
		warnWriter = openLogWriter("warn.log")
	})
	return warnWriter
}

func openLogWriter(path string) io.Writer {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// Container filesystems are sometimes read-only; stdout alone is
		// enough for k8s log aggregation to pick up the line.
		logs.Info("log file unavailable, using stdout only: " + path)
		return os.Stdout
	}
	return io.MultiWriter(os.Stdout, f)
}

// formatter is set once and reused — building a JSONFormatter on every call
// (the prior pattern) was harmless but wasteful.
var formatter = &logs.JSONFormatter{}
