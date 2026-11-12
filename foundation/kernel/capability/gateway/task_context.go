package gateway

import "context"

// TaskReference is supplied only by trusted runners, never request payloads.
type TaskReference struct {
	RunID                   int64
	TenantID                string
	AccountID, DepartmentID int64
}
type taskReferenceKey struct{}

func WithTaskReference(ctx context.Context, reference TaskReference) context.Context {
	return context.WithValue(ctx, taskReferenceKey{}, reference)
}
func CurrentTaskReference(ctx context.Context) (TaskReference, bool) {
	value, ok := ctx.Value(taskReferenceKey{}).(TaskReference)
	return value, ok
}
