# Example: marshal

Demonstrates `SLogMarshaler` — the interface for types that want full control over their log representation, analogous to `json.Marshaler`.

```go
type SLogMarshaler interface {
    MarshalSLog() []slog.Attr
}
```

Any type implementing this interface is handled automatically by `ToAttrs`, both at the top level and as a nested field inside another struct.

## Run

```sh
go run github.com/silvercory/slog-schema/example/marshal
```

## Output

```
level=INFO msg="order created" id=ord_abc123 total.amount="49.99 GBP" status.code=201 status.label=Created labels.0=new labels.1=priority labels.2=gift
level=INFO msg=payment amount="9.00 USD"
```

## Types in this example

**`Money`** — formats a monetary amount as a human-readable string rather than exposing raw integer fields:
```go
func (m Money) MarshalSLog() []slog.Attr {
    return []slog.Attr{
        slog.String("amount", fmt.Sprintf("%d.%02d %s", m.Units, m.Cents, m.Currency)),
    }
}
```

**`Status`** — maps an integer HTTP status code to both the numeric value and a human-readable label:
```go
func (s Status) MarshalSLog() []slog.Attr {
    return []slog.Attr{
        slog.Int("code", int(s)),
        slog.String("label", name),
    }
}
```

**`Tags`** — collapses a string slice into a single comma-separated value instead of indexed group entries:
```go
func (t Tags) MarshalSLog() []slog.Attr {
    return []slog.Attr{
        slog.String("tags", strings.Join(t, ",")),
    }
}
```
