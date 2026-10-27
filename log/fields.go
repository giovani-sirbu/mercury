package log

import (
	"context"

	logs "github.com/sirupsen/logrus"
)

// Field is a structured key/value pair on one log line; build it with the helpers below.
type Field struct {
	Key   string
	Value string
}

// correlationIDFieldKey is the JSON key of the correlation id; dashboards group by it.
const correlationIDFieldKey = "correlation_id"

// correlationIDContextKey is the typed context key of the correlation id.
type correlationIDContextKey struct{}

// WithCorrelation tags a log line with the per-request correlation id.
func WithCorrelation(id string) Field {
	return Field{Key: correlationIDFieldKey, Value: id}
}

// ContextWithCorrelation returns a child context carrying the correlation id.
func ContextWithCorrelation(parent context.Context, id string) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	if id == "" {
		return parent
	}
	return context.WithValue(parent, correlationIDContextKey{}, id)
}

// CorrelationFromContext returns the context's correlation id, or "" when absent.
func CorrelationFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(correlationIDContextKey{}).(string); ok {
		return v
	}
	return ""
}

// applyFields merges the Field entries into base, dropping empty values.
func applyFields(base logs.Fields, fields ...Field) logs.Fields {
	for _, f := range fields {
		if f.Key == "" || f.Value == "" {
			continue
		}
		base[f.Key] = f.Value
	}
	return base
}
