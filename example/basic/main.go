// basic demonstrates core slog-schema features: struct tags, embedded structs,
// omitempty, slices, maps, and nested structs.
package main

import (
	"log/slog"
	"os"
	"time"

	slogschema "github.com/silvercory/slog-schema"
)

// Meta is an embedded struct — its fields are inlined into the parent's log output.
type Meta struct {
	TraceID string `slog:"trace_id"`
	Region  string `slog:"region"`
}

// Address is a nested struct — it appears as a slog group.
type Address struct {
	Street string `slog:"street"`
	City   string `slog:"city"`
	Post   string `slog:"post_code"`
}

type Request struct {
	Meta                       // inlined
	Method   string            `slog:"method"`
	Path     string            `slog:"path"`
	Status   int               `slog:"status"`
	Duration time.Duration     `slog:"duration"`
	Error    string            `slog:"error,omitempty"`   // omitted when empty
	Tags     []string          `slog:"tags"`
	Headers  map[string]string `slog:"headers"`
	Addr     Address           `slog:"addr"`
	Internal string            // no slog tag — skipped entirely
	Secret   string            `slog:"-"` // explicitly skipped
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Error is empty — omitempty suppresses it.
	req := Request{
		Meta:     Meta{TraceID: "abc-123", Region: "eu-west-1"},
		Method:   "POST",
		Path:     "/api/orders",
		Status:   201,
		Duration: 18 * time.Millisecond,
		Tags:     []string{"orders", "v2"},
		Headers:  map[string]string{"content-type": "application/json"},
		Addr:     Address{Street: "1 High St", City: "London", Post: "EC1A 1BB"},
		Internal: "not logged",
		Secret:   "not logged either",
	}

	logger.LogAttrs(nil, slog.LevelInfo, "request", slogschema.ToAttrs(req)...)

	// With an error present.
	req.Status = 500
	req.Error = "upstream timeout"
	logger.LogAttrs(nil, slog.LevelError, "request", slogschema.ToAttrs(req)...)
}
