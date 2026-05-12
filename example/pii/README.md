# Example: pii

Demonstrates hashed fields for personally identifiable information (PII).

Fields tagged `hashed` have their value replaced with a SHA-256 digest before logging. The hash is deterministic — the same input always produces the same digest — so log lines for the same user can be correlated without ever writing raw PII to a log sink.

## Run

```sh
go run github.com/silvercory/slog-schema/example/pii
```

## Output

```
level=INFO msg=login  id=sha256:fe675f... email=sha256:ff8d98... phone=sha256:d867b6... role=admin
level=WARN msg=audit  action=export_data user.id=sha256:fe675f... user.email=sha256:ff8d98... user.phone=sha256:d867b6... user.role=admin
level=INFO msg=login  id=sha256:fe675f... email=sha256:ff8d98... role=admin
```

Notice:
- `id`, `email`, and `phone` are hashed on every line — raw values never appear
- The hashes are identical across the `login` and `audit` lines for the same user, enabling correlation
- `phone` is absent from the third line (`omitempty,hashed` — skipped when blank)
- `role` is plain text — not PII

## Tags used

```go
ID    uint64 `slog:"id,hashed"`
Email string `slog:"email,hashed"`
Phone string `slog:"phone,omitempty,hashed"`
Role  string `slog:"role"`
```
