// pii demonstrates hashed fields for PII data such as emails, phone numbers,
// and user identifiers. Hashed values are deterministic SHA-256 digests, so
// they can be correlated across log lines without exposing raw PII.
package main

import (
	"log/slog"
	"os"

	slogschema "github.com/silvercory/slog-schema"
)

type User struct {
	// ID is hashed — logged as sha256:<hex> so it can still be correlated.
	ID uint64 `slog:"id,hashed"`

	// Email and Phone are PII — always hashed.
	Email string `slog:"email,hashed"`
	Phone string `slog:"phone,omitempty,hashed"` // also omitted when blank

	// Role is not PII — logged in plain text.
	Role string `slog:"role"`
}

type AuditEvent struct {
	Action string `slog:"action"`
	User   User   `slog:"user"`
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Two log lines for the same user will produce identical hashes,
	// so they can be correlated without ever logging the raw values.
	u := User{
		ID:    1001,
		Email: "alice@example.com",
		Phone: "+447700900000",
		Role:  "admin",
	}

	logger.LogAttrs(nil, slog.LevelInfo, "login", slogschema.ToAttrs(u)...)

	evt := AuditEvent{
		Action: "export_data",
		User:   u,
	}
	logger.LogAttrs(nil, slog.LevelWarn, "audit", slogschema.ToAttrs(evt)...)

	// Phone omitted when blank.
	u.Phone = ""
	logger.LogAttrs(nil, slog.LevelInfo, "login", slogschema.ToAttrs(u)...)
}
