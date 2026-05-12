# protoc-gen-slogschema

A `protoc` plugin that generates `MarshalSLog() []slog.Attr` methods on your protoc-gen-go message types, making them directly compatible with [slog-schema](../../README.md).

The same Go struct is used for both gRPC communication and structured logging — no separate logging struct, no manual field mapping.

## Installation

```sh
go install github.com/silvercory/slog-schema/cmd/protoc-gen-slogschema@latest
```

## Usage

Run alongside `protoc-gen-go` and `protoc-gen-connect-go`:

```sh
protoc \
  --go_out=. \
  --slogschema_out=. \
  --proto_path=. \
  path/to/your.proto
```

This generates two files per `.proto`:

| File | Contents |
|------|----------|
| `your.pb.go` | Standard protoc-gen-go types |
| `your.slogschema.pb.go` | `MarshalSLog()` methods in the same package |

Because `*YourMessage` now implements `slogschema.SLogMarshaler`, you pass it directly to `ToAttrs`:

```go
user, _ := userService.GetUser(ctx, req)
logger.LogAttrs(ctx, slog.LevelInfo, "user fetched", slogschema.ToAttrs(user)...)
```

## Annotations

Import the options file in your proto:

```protobuf
import "slogschema/v1/options.proto";
```

### Message-level

```protobuf
message Payment {
  option (slogschema.v1.message) = {default_skip: true};

  // Only annotated fields are logged — everything else is skipped.
  string id          = 1 [(slogschema.v1.field) = {omitempty: true}];
  string card_number = 2 [(slogschema.v1.field) = {hashed: true}];
  string internal_ref = 3; // no annotation — skipped
}
```

| Option | Effect |
|--------|--------|
| `default_skip: true` | All fields are skipped unless they carry an explicit field annotation |

### Field-level

```protobuf
message User {
  uint64 id           = 1 [(slogschema.v1.field) = {hashed: true, omitempty: true}];
  string email        = 2 [(slogschema.v1.field) = {hashed: true}];
  string display_name = 3 [(slogschema.v1.field) = {name: "name"}];
  string password     = 4 [(slogschema.v1.field) = {skip: true}];
  string role         = 5; // logged as-is (unless message has default_skip)
}
```

| Option | Effect |
|--------|--------|
| `name` | Override the slog attribute key |
| `hashed` | Replace the value with its SHA-256 hash |
| `omitempty` | Omit the field when it is a zero value |
| `skip` | Exclude this field entirely |

Options can be combined: `{name: "card", hashed: true, omitempty: true}`.

## Type mapping

| Proto type | Generated Go expression |
|------------|------------------------|
| `string` | `slog.StringValue(x.GetField())` |
| `bool` | `slog.BoolValue(x.GetField())` |
| `int32`, `sint32`, `sfixed32` | `slog.Int64Value(int64(x.GetField()))` |
| `int64`, `sint64`, `sfixed64` | `slog.Int64Value(x.GetField())` |
| `uint32`, `fixed32` | `slog.Uint64Value(uint64(x.GetField()))` |
| `uint64`, `fixed64` | `slog.Uint64Value(x.GetField())` |
| `float` | `slog.Float64Value(float64(x.GetField()))` |
| `double` | `slog.Float64Value(x.GetField())` |
| `bytes` | `slog.StringValue(fmt.Sprintf("%x", x.GetField()))` |
| `enum` | `slog.Int64Value(int64(x.GetField()))` |
| `google.protobuf.Timestamp` | `slog.TimeValue(x.GetField().AsTime())` |
| `google.protobuf.Duration` | `slog.DurationValue(x.GetField().AsDuration())` |
| Wrapper types (`StringValue` etc.) | Unwrapped to native Go type |
| Nested message | `slog.GroupValue(x.GetField().MarshalSLog()...)` |
| `repeated` | `slog.GroupValue` with `"0"`, `"1"` … keys |
| `oneof` | Runtime type switch; only the set variant is emitted |

## oneof fields

Each `oneof` variant becomes a case in a type switch at runtime — only the set variant appears in the log output:

```protobuf
message Notification {
  oneof channel {
    string email = 1;
    string sms   = 2;
  }
}
```

```
# email set:   channel=alerts@example.com  (key taken from field name)
# sms set:     channel=+447700900000
# neither set: (attr omitted)
```

Individual variants can be annotated or skipped independently:

```protobuf
oneof channel {
  string email = 1 [(slogschema.v1.field) = {hashed: true}];
  string sms   = 2 [(slogschema.v1.field) = {skip: true}];
}
```

## Nested messages

Nested messages get their own `MarshalSLog()` method and are emitted as a `slog.Group`:

```
address.street=... address.city=... address.country_code=GB
```

Nil message fields are emitted as an empty group (or suppressed entirely with `omitempty`).

## default_skip pattern

For sensitive messages where you want to opt fields *in* to logging rather than out, set `default_skip: true` on the message and annotate only the fields you want logged:

```protobuf
message PaymentMethod {
  option (slogschema.v1.message) = {default_skip: true};

  string id       = 1 [(slogschema.v1.field) = {}];          // opted in, logged as-is
  string type     = 2 [(slogschema.v1.field) = {name: "kind"}]; // opted in, custom key
  string pan      = 3 [(slogschema.v1.field) = {hashed: true}]; // opted in, hashed
  string cvv      = 4; // no annotation — skipped entirely
  string raw_data = 5; // no annotation — skipped entirely
}
```

An annotation with no options (`{}`) is enough to opt a field in.
