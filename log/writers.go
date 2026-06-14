package log

import (
	"io"
	"os"

	logs "github.com/sirupsen/logrus"
)

// Logging now goes to stdout only. Promtail (running as a sidecar on every
// VM) tails the Docker container's stdout and ships every JSON line to Loki
// with parsed labels (level, span, track, parent, correlation_id). The
// previous file-writer path (error.log / info.log / warn.log inside the
// container's WORKDIR) was operationally dead weight: the files lived on
// ephemeral container storage with no rotation, contributed nothing to
// observability, and ran the risk of filling disk over time.
//
// Keeping the *Writer accessors so the public API of this package is
// unchanged; each one returns os.Stdout. If someone later wants a per-level
// file fallback (for development outside Docker), reintroduce a build-tagged
// alternative.
func getErrorWriter() io.Writer { return os.Stdout }
func getInfoWriter() io.Writer  { return os.Stdout }
func getWarnWriter() io.Writer  { return os.Stdout }

// formatter is set once and reused — building a JSONFormatter on every call
// (the prior pattern) was harmless but wasteful.
var formatter = &logs.JSONFormatter{}
