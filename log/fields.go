package log

import (
	"context"

	logs "github.com/sirupsen/logrus"
)

// Field is a structured key/value pair attached to a single log line.
// Use the helpers below (WithCorrelation, etc.) rather than building one
// by hand so the keys stay consistent across services and dashboards.
type Field struct {
	Key   string
	Value string
}

// correlationIDFieldKey is the JSON key used in structured log output.
// Dashboards group by this string; keep it stable.
const correlationIDFieldKey = "correlation_id"

// correlationIDContextKey is the unexported typed key used to store the
// correlation id on context.Context. A typed key is the documented Go idiom
// — string keys are flagged by `go vet` and can collide with unrelated
// packages that also key by "correlation_id".
type correlationIDContextKey struct{}

// WithCorrelation tags a log line with the per-request correlation id.
// Pair with Error / Info / Warn:
//
//	log.Info("trade created", "CreateTrade", "Handler", log.WithCorrelation(cid))
func WithCorrelation(id string) Field {
	return Field{Key: correlationIDFieldKey, Value: id}
}

// ContextWithCorrelation returns a child context carrying the correlation id.
// Used by the messagebroker consumer (and anywhere else that derives a
// context from an incoming source) so downstream code can retrieve the id
// via CorrelationFromContext.
func ContextWithCorrelation(parent context.Context, id string) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	if id == "" {
		return parent
	}
	return context.WithValue(parent, correlationIDContextKey{}, id)
}

// CorrelationFromContext pulls a correlation id off a context.Context if one
// was set by ContextWithCorrelation. Returns "" when absent.
func CorrelationFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(correlationIDContextKey{}).(string); ok {
		return v
	}
	return ""
}

// applyFields merges variadic Field entries into a logrus.Fields map alongside
// the standard span/track/parent fields. Empty values are dropped so a missing
// correlation id does not pollute the structured output.
func applyFields(base logs.Fields, fields ...Field) logs.Fields {
	for _, f := range fields {
		if f.Key == "" || f.Value == "" {
			continue
		}
		base[f.Key] = f.Value
	}
	return base
}
