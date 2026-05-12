// protobuf demonstrates Option A: protoc-gen-slogschema generates a
// MarshalSLog() method directly on the protoc-gen-go *User type.
//
// There is one struct used for both gRPC communication and logging — no
// mapping required. Passing *User to slog-schema's ToAttrs works because
// *User now implements SLogMarshaler.
//
// The generated file is user_slogschema.pb.go, produced by:
//
//	protoc --slogschema_out=. user.proto
package main

import (
	"log/slog"
	"os"
	"time"

	slogschema "github.com/silvercory/slog-schema"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	u := &User{
		Id:          42,
		Email:       "bob@example.com",
		DisplayName: "Bob Smith",
		Phone:       "+447700900001",
		Role:        "editor",
		PasswordHash: "bcrypt:$2a$...", // skip annotated — never appears in logs
		CreatedAt:   timestamppb.New(time.Date(2024, 3, 15, 9, 0, 0, 0, time.UTC)),
		Address: &Address{
			Street:  "10 Downing St",
			City:    "London",
			Country: "GB",
		},
		NotificationMethod: &User_NotifyEmail{NotifyEmail: "alerts@example.com"},
		Roles:              []string{"read", "write"},
	}

	// *User implements SLogMarshaler — ToAttrs calls MarshalSLog() directly.
	// id, email, phone are hashed; password_hash is absent entirely.
	logger.LogAttrs(nil, slog.LevelInfo, "user loaded", slogschema.ToAttrs(u)...)

	// Works with logger.Error via ToAttrsAny.
	u.Role = "suspended"
	logger.Error("user suspended", slogschema.ToAttrsAny(u)...)

	// Switching the oneof — only the set variant appears in the log.
	u.NotificationMethod = &User_NotifySms{NotifySms: "+447700900001"}
	logger.LogAttrs(nil, slog.LevelInfo, "notification method changed", slogschema.ToAttrs(u)...)

	// Nil address — omitempty suppresses it.
	u.Address = nil
	logger.LogAttrs(nil, slog.LevelInfo, "user updated", slogschema.ToAttrs(u)...)
}
