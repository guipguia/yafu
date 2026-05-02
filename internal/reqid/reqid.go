// Package reqid carries a per-request correlation ID through context so
// the access log, audit log, metrics, and downstream handlers all share
// one id without forcing import cycles between server / api / audit.
package reqid

import "context"

type ctxKey int

const key ctxKey = iota

// With attaches id to ctx.
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, key, id)
}

// From returns the id attached to ctx, or "" when absent.
func From(ctx context.Context) string {
	if v, ok := ctx.Value(key).(string); ok {
		return v
	}
	return ""
}
