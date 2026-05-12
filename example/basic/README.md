# Example: basic

Demonstrates the core features of slog-schema:

- Struct tags (`slog:"name"`)
- Fields with no tag or `slog:"-"` are skipped
- `omitempty` — field absent when zero
- Embedded structs — fields inlined at the top level
- Nested structs — emitted as a `slog.Group`
- Slices — emitted as a group with `"0"`, `"1"` … keys
- Maps — emitted as a group with stringified keys

## Run

```sh
go run github.com/silvercory/slog-schema/example/basic
```

## Output

```
level=INFO  msg=request trace_id=abc-123 region=eu-west-1 method=POST path=/api/orders status=201 duration=18ms tags.0=orders tags.1=v2 headers.content-type=application/json addr.street="1 High St" addr.city=London addr.post_code="EC1A 1BB"
level=ERROR msg=request trace_id=abc-123 region=eu-west-1 method=POST path=/api/orders status=500 duration=18ms error="upstream timeout" tags.0=orders tags.1=v2 headers.content-type=application/json addr.street="1 High St" addr.city=London addr.post_code="EC1A 1BB"
```

Notice:
- `trace_id` and `region` are inlined from the embedded `Meta` struct
- `error` is absent from the first line (`omitempty`) and present in the second
- `addr` fields are grouped under the `addr.` prefix
- `internal` and `secret` never appear
