# Example: protobuf

Demonstrates using `protoc-gen-slogschema` with a Connect-RPC / gRPC service.

The plugin generates a `MarshalSLog() []slog.Attr` method directly on each protoc-gen-go message type. This means:

- **One struct** for both wire transport and logging — no separate logging struct, no field mapping
- Annotations in the `.proto` file control logging behaviour at the source of truth
- `*User` implements `slogschema.SLogMarshaler` automatically — pass it straight to `ToAttrs`

## Files

| File | Origin | Description |
|------|--------|-------------|
| `user.proto` | Hand-written | Proto definition with slogschema annotations |
| `user.pb.go` | `protoc-gen-go` (simulated) | Standard Go message types and getters |
| `user.slogschema.pb.go` | `protoc-gen-slogschema` (simulated) | Generated `MarshalSLog()` methods |
| `main.go` | Hand-written | Usage example |

In a real project `user.pb.go` and `user_slogschema.pb.go` are both generated — you only maintain `user.proto`.

## Run

```sh
go run github.com/silvercory/slog-schema/example/protobuf
```

## Output

```
level=INFO  msg="user loaded"               id=sha256:... email=sha256:... name="Bob Smith" phone=sha256:... role=editor created_at=2024-03-15T09:00:00Z address.street="10 Downing St" address.city=London address.country_code=GB notify_email=alerts@example.com roles.0=read roles.1=write
level=ERROR msg="user suspended"            id=sha256:... email=sha256:... name="Bob Smith" phone=sha256:... role=suspended ...
level=INFO  msg="notification method changed" ... notify_sms=+447700900001 ...
level=INFO  msg="user updated"              ... (address absent)
```

Notice:
- `password_hash` never appears — `skip: true` in the proto means it is absent from `MarshalSLog` entirely
- `id`, `email`, `phone` are hashed on every line
- `display_name` is logged as `name` (custom key via `name: "name"` annotation)
- `notify_email` / `notify_sms` — only the set `oneof` variant appears; it switches on the third line
- `address` disappears on the last line (`omitempty` + nil pointer)

## Annotations used in user.proto

```protobuf
message User {
  uint64 id           = 1 [(slogschema.v1.field) = {hashed: true, omitempty: true}];
  string email        = 2 [(slogschema.v1.field) = {hashed: true}];
  string display_name = 3 [(slogschema.v1.field) = {name: "name"}];
  string phone        = 4 [(slogschema.v1.field) = {hashed: true, omitempty: true}];
  string role         = 5;
  string password_hash = 6 [(slogschema.v1.field) = {skip: true}];
  ...
}
```

## Generating in a real project

Install the plugin and run protoc alongside your existing generators:

```sh
go install github.com/silvercory/slog-schema/cmd/protoc-gen-slogschema@latest

protoc \
  --go_out=. \
  --go-grpc_out=. \
  --slogschema_out=. \
  --proto_path=. \
  api/user.proto
```

With buf:

```yaml
# buf.gen.yaml
plugins:
  - plugin: go
    out: gen
  - plugin: connect-go
    out: gen
  - plugin: slogschema
    out: gen
```
