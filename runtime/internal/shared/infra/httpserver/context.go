package httpserver

import "context"

type ctxKey string

const requestIDKey ctxKey = "request_id"

// WithRequestID stores id on ctx for logs and access records.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFrom is empty when RequestID middleware has not run.
func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}
