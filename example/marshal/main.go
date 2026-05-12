// marshal demonstrates SLogMarshaler — a type that fully controls its own
// slog representation, analogous to json.Marshaler.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	slogschema "github.com/silvercory/slog-schema"
)

// Money formats a monetary amount as "<units> <currency>" rather than
// exposing raw integer fields which would be meaningless in a log.
type Money struct {
	Units    int64
	Cents    int64
	Currency string
}

func (m Money) MarshalSLog() []slog.Attr {
	return []slog.Attr{
		slog.String("amount", fmt.Sprintf("%d.%02d %s", m.Units, m.Cents, m.Currency)),
	}
}

// Status wraps an integer code and provides a human-readable label.
type Status int

func (s Status) MarshalSLog() []slog.Attr {
	label := map[Status]string{
		200: "OK",
		201: "Created",
		400: "Bad Request",
		404: "Not Found",
		500: "Internal Server Error",
	}
	name, ok := label[s]
	if !ok {
		name = "Unknown"
	}
	return []slog.Attr{
		slog.Int("code", int(s)),
		slog.String("label", name),
	}
}

// Tags wraps a string slice and logs it as a single comma-separated value.
type Tags []string

func (t Tags) MarshalSLog() []slog.Attr {
	return []slog.Attr{
		slog.String("tags", strings.Join(t, ",")),
	}
}

type Order struct {
	ID     string  `slog:"id"`
	Total  Money   `slog:"total"`
	Status Status  `slog:"status"`
	Labels Tags    `slog:"labels"`
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	order := Order{
		ID:     "ord_abc123",
		Total:  Money{Units: 49, Cents: 99, Currency: "GBP"},
		Status: 201,
		Labels: Tags{"new", "priority", "gift"},
	}

	logger.LogAttrs(nil, slog.LevelInfo, "order created", slogschema.ToAttrs(order)...)

	// MarshalSLog also works at the top level.
	logger.LogAttrs(nil, slog.LevelInfo, "payment", slogschema.ToAttrs(Money{Units: 9, Cents: 0, Currency: "USD"})...)
}
