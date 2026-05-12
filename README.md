# slog-schema

A Go library for structured logging with `log/slog` using struct tags. Define your log schema once on a struct and pass it directly to any slog logger — no manual attribute building.

## Installation

```sh
go get github.com/silvercory/slog-schema
```

## Quick start

```go
type Request struct {
    TraceID  string        `slog:"trace_id"`
    Method   string        `slog:"method"`
    Path     string        `slog:"path"`
    Status   int           `slog:"status"`
    Duration time.Duration `slog:"duration"`
    Error    string        `slog:"error,omitempty"`
    Email    string        `slog:"email,hashed"`
}

req := Request{
    TraceID:  "abc-123",
    Method:   "POST",
    Path:     "/api/orders",
    Status:   201,
    Duration: 18 * time.Millisecond,
    Email:    "alice@example.com",
}

logger.LogAttrs(ctx, slog.LevelInfo, "request", slogschema.ToAttrs(req)...)
// time=... level=INFO msg=request trace_id=abc-123 method=POST path=/api/orders status=201 duration=18ms email=sha256:...
```

## Struct tags

| Tag | Effect |
|-----|--------|
| `slog:"name"` | Log this field with the given key |
| `slog:"-"` | Skip this field entirely |
| _(no tag)_ | Skip this field entirely |
| `slog:"name,omitempty"` | Skip when the value is a zero value |
| `slog:"name,hashed"` | Replace the value with its SHA-256 hash |
| `slog:"name,omitempty,hashed"` | Both — skip if zero, hash otherwise |

## Supported types

All types natively supported by `log/slog` are handled directly:

| Go type | slog kind |
|---------|-----------|
| `string` | `StringValue` |
| `bool` | `BoolValue` |
| `int`, `int8` … `int64` | `Int64Value` |
| `uint`, `uint8` … `uint64` | `Uint64Value` |
| `float32`, `float64` | `Float64Value` |
| `time.Time` | `TimeValue` |
| `time.Duration` | `DurationValue` |
| Pointer | Dereferenced; nil → `AnyValue(nil)` |
| Struct | `GroupValue` (recursive) |
| Slice / Array | `GroupValue` with `"0"`, `"1"` … keys |
| Map | `GroupValue` with stringified keys |
| `slog.LogValuer` | `.LogValue()` called |
| Everything else | `AnyValue` |

## Embedded structs

Embedded structs without a tag are inlined — their fields appear at the top level:

```go
type Meta struct {
    TraceID string `slog:"trace_id"`
    Region  string `slog:"region"`
}

type Request struct {
    Meta            // inlined
    Method string   `slog:"method"`
}

// Output: trace_id=... region=... method=...
```

Give the embedded field a tag to emit it as a named group instead:

```go
type Request struct {
    Meta   `slog:"meta"` // grouped
    Method string        `slog:"method"`
}

// Output: meta.trace_id=... meta.region=... method=...
```

## PII and hashed fields

Use `hashed` for PII such as emails, phone numbers, and user identifiers. The SHA-256 digest is deterministic, so the same value produces the same hash across log lines — enabling correlation without exposing raw data.

```go
type User struct {
    ID    uint64 `slog:"id,hashed"`
    Email string `slog:"email,hashed"`
    Role  string `slog:"role"`
}
```

## Custom marshalling

Implement `SLogMarshaler` to take full control of how a type is logged:

```go
type Money struct {
    Units    int64
    Currency string
}

func (m Money) MarshalSLog() []slog.Attr {
    return []slog.Attr{
        slog.String("amount", fmt.Sprintf("%d %s", m.Units, m.Currency)),
    }
}
```

Any type implementing `SLogMarshaler` is handled automatically, both at the top level and as a nested field.

## API

```go
// ToAttrs converts a struct to []slog.Attr for use with logger.LogAttrs.
func ToAttrs(v any) []slog.Attr

// ToAttrsAny converts a struct to []any for use with logger.Info, logger.Error, etc.
func ToAttrsAny(v any) []any
```

## Protobuf

`protoc-gen-slogschema` generates `MarshalSLog()` methods directly on your protoc-gen-go message types, so the same struct is used for both gRPC and logging with no mapping required.

See [cmd/protoc-gen-slogschema](cmd/protoc-gen-slogschema/README.md) for details.

## Examples

- [basic](example/basic/) — tags, omitempty, embedded structs, slices, maps
- [pii](example/pii/) — hashed fields for PII
- [marshal](example/marshal/) — custom `MarshalSLog` implementations
- [protobuf](example/protobuf/) — generated methods on protoc-gen-go types
