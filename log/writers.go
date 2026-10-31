package log

import (
	"io"
	"os"

	logs "github.com/sirupsen/logrus"
)

// Every level logs to stdout: Promtail tails the container and ships each
// JSON line to Loki. The per-level accessors stay so the package API holds.
func getErrorWriter() io.Writer { return os.Stdout }
func getInfoWriter() io.Writer  { return os.Stdout }
func getWarnWriter() io.Writer  { return os.Stdout }

// formatter is built once and reused.
var formatter = &logs.JSONFormatter{}
