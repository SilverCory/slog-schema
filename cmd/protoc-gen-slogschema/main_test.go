package main

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	pluginpb "google.golang.org/protobuf/types/pluginpb"
)

// --- helpers ---

func makeRequest(files ...*descriptorpb.FileDescriptorProto) *pluginpb.CodeGeneratorRequest {
	req := &pluginpb.CodeGeneratorRequest{ProtoFile: files}
	for _, f := range files {
		req.FileToGenerate = append(req.FileToGenerate, f.GetName())
	}
	return req
}

func simpleFile(name, pkg string, msgs ...*descriptorpb.DescriptorProto) *descriptorpb.FileDescriptorProto {
	syntax := "proto3"
	return &descriptorpb.FileDescriptorProto{
		Name:        proto.String(name),
		Package:     proto.String(pkg),
		Syntax:      &syntax,
		MessageType: msgs,
		Options:     &descriptorpb.FileOptions{GoPackage: proto.String(pkg)},
	}
}

func field(name string, typ descriptorpb.FieldDescriptorProto_Type, num int32) *descriptorpb.FieldDescriptorProto {
	label := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(num),
		Type:   &typ,
		Label:  &label,
	}
}

func repeatedField(name string, typ descriptorpb.FieldDescriptorProto_Type, num int32) *descriptorpb.FieldDescriptorProto {
	f := field(name, typ, num)
	label := descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	f.Label = &label
	return f
}

func msgField(name, typeName string, num int32) *descriptorpb.FieldDescriptorProto {
	f := field(name, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, num)
	f.TypeName = proto.String(typeName)
	return f
}

func withSlogOpts(f *descriptorpb.FieldDescriptorProto, name string, hashed, omitempty, skip bool) *descriptorpb.FieldDescriptorProto {
	var inner []byte
	if name != "" {
		inner = protowire.AppendTag(inner, 1, protowire.BytesType)
		inner = protowire.AppendString(inner, name)
	}
	if hashed {
		inner = protowire.AppendTag(inner, 2, protowire.VarintType)
		inner = protowire.AppendVarint(inner, 1)
	}
	if omitempty {
		inner = protowire.AppendTag(inner, 3, protowire.VarintType)
		inner = protowire.AppendVarint(inner, 1)
	}
	if skip {
		inner = protowire.AppendTag(inner, 4, protowire.VarintType)
		inner = protowire.AppendVarint(inner, 1)
	}
	var ext []byte
	ext = protowire.AppendTag(ext, 77148, protowire.BytesType)
	ext = protowire.AppendBytes(ext, inner)
	if f.Options == nil {
		f.Options = &descriptorpb.FieldOptions{}
	}
	f.Options.ProtoReflect().SetUnknown(ext)
	return f
}

func getFile(t *testing.T, resp *pluginpb.CodeGeneratorResponse, suffix string) string {
	t.Helper()
	if resp.Error != nil {
		t.Fatalf("generator error: %s", resp.GetError())
	}
	for _, f := range resp.File {
		if strings.HasSuffix(f.GetName(), suffix) {
			return f.GetContent()
		}
	}
	t.Fatalf("no output file with suffix %q; files: %v", suffix, fileNames(resp))
	return ""
}

func fileNames(resp *pluginpb.CodeGeneratorResponse) []string {
	names := make([]string, len(resp.File))
	for i, f := range resp.File {
		names[i] = f.GetName()
	}
	return names
}

func assertContains(t *testing.T, src, want string) {
	t.Helper()
	if !strings.Contains(src, want) {
		t.Errorf("expected output to contain %q\ngot:\n%s", want, src)
	}
}

func assertNotContains(t *testing.T, src, want string) {
	t.Helper()
	if strings.Contains(src, want) {
		t.Errorf("expected output NOT to contain %q\ngot:\n%s", want, src)
	}
}

// --- tests ---

func TestOutputFilename(t *testing.T) {
	cases := []struct{ in, want string }{
		{"foo/bar.proto", "foo/bar.slogschema.pb.go"},
		{"baz.proto", "baz.slogschema.pb.go"},
	}
	for _, c := range cases {
		if got := outputFilename(c.in); got != c.want {
			t.Errorf("outputFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCamelCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"foo_bar", "FooBar"},
		{"user_id", "UserId"},
		{"name", "Name"},
		{"first_name_last", "FirstNameLast"},
	}
	for _, c := range cases {
		if got := camelCase(c.in); got != c.want {
			t.Errorf("camelCase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMethodGenerated(t *testing.T) {
	msg := &descriptorpb.DescriptorProto{
		Name: proto.String("User"),
		Field: []*descriptorpb.FieldDescriptorProto{
			field("name", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1),
			field("age", descriptorpb.FieldDescriptorProto_TYPE_INT32, 2),
		},
	}
	src := getFile(t, generate(makeRequest(simpleFile("user.proto", "userpb", msg))), ".slogschema.pb.go")

	assertContains(t, src, "func (x *User) MarshalSLog() []slog.Attr")
	assertContains(t, src, `"name"`)
	assertContains(t, src, "GetName()")
	assertContains(t, src, `"age"`)
	assertContains(t, src, "GetAge()")
}

func TestNilGuard(t *testing.T) {
	msg := &descriptorpb.DescriptorProto{
		Name:  proto.String("User"),
		Field: []*descriptorpb.FieldDescriptorProto{field("name", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1)},
	}
	src := getFile(t, generate(makeRequest(simpleFile("user.proto", "userpb", msg))), ".slogschema.pb.go")
	assertContains(t, src, "if x == nil")
}

func TestSkipField(t *testing.T) {
	msg := &descriptorpb.DescriptorProto{
		Name: proto.String("User"),
		Field: []*descriptorpb.FieldDescriptorProto{
			field("name", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1),
			withSlogOpts(field("secret", descriptorpb.FieldDescriptorProto_TYPE_STRING, 2), "", false, false, true),
		},
	}
	src := getFile(t, generate(makeRequest(simpleFile("user.proto", "userpb", msg))), ".slogschema.pb.go")
	assertContains(t, src, "GetName()")
	assertNotContains(t, src, "GetSecret()")
}

func TestHashedField(t *testing.T) {
	msg := &descriptorpb.DescriptorProto{
		Name: proto.String("User"),
		Field: []*descriptorpb.FieldDescriptorProto{
			withSlogOpts(field("email", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1), "", true, false, false),
		},
	}
	src := getFile(t, generate(makeRequest(simpleFile("user.proto", "userpb", msg))), ".slogschema.pb.go")
	assertContains(t, src, "sha256")
	assertContains(t, src, `"email"`)
	assertContains(t, src, "GetEmail()")
}

func TestOmitemptyField(t *testing.T) {
	msg := &descriptorpb.DescriptorProto{
		Name: proto.String("User"),
		Field: []*descriptorpb.FieldDescriptorProto{
			withSlogOpts(field("nickname", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1), "", false, true, false),
		},
	}
	src := getFile(t, generate(makeRequest(simpleFile("user.proto", "userpb", msg))), ".slogschema.pb.go")
	assertContains(t, src, `"nickname"`)
	assertContains(t, src, "isEmpty")
	assertContains(t, src, "GetNickname()")
}

func TestCustomName(t *testing.T) {
	msg := &descriptorpb.DescriptorProto{
		Name: proto.String("User"),
		Field: []*descriptorpb.FieldDescriptorProto{
			withSlogOpts(field("user_identifier", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1), "uid", false, false, false),
		},
	}
	src := getFile(t, generate(makeRequest(simpleFile("user.proto", "userpb", msg))), ".slogschema.pb.go")
	assertContains(t, src, `"uid"`)
	assertNotContains(t, src, `"user_identifier"`)
}

func TestAllOptionsCombined(t *testing.T) {
	msg := &descriptorpb.DescriptorProto{
		Name: proto.String("Payment"),
		Field: []*descriptorpb.FieldDescriptorProto{
			withSlogOpts(field("card_number", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1), "card", true, true, false),
		},
	}
	src := getFile(t, generate(makeRequest(simpleFile("payment.proto", "paymentpb", msg))), ".slogschema.pb.go")
	assertContains(t, src, `"card"`)
	assertContains(t, src, "sha256")
	assertContains(t, src, "isEmpty")
}

func TestRepeatedField(t *testing.T) {
	msg := &descriptorpb.DescriptorProto{
		Name: proto.String("Post"),
		Field: []*descriptorpb.FieldDescriptorProto{
			repeatedField("tags", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1),
		},
	}
	src := getFile(t, generate(makeRequest(simpleFile("post.proto", "postpb", msg))), ".slogschema.pb.go")
	assertContains(t, src, "GetTags()")
	assertContains(t, src, "strconv.Itoa(i)")
	assertContains(t, src, "slog.GroupValue")
}

func TestNestedMessage(t *testing.T) {
	addr := &descriptorpb.DescriptorProto{
		Name:  proto.String("Address"),
		Field: []*descriptorpb.FieldDescriptorProto{field("city", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1)},
	}
	user := &descriptorpb.DescriptorProto{
		Name:  proto.String("User"),
		Field: []*descriptorpb.FieldDescriptorProto{msgField("address", ".userpb.Address", 2)},
	}
	src := getFile(t, generate(makeRequest(simpleFile("user.proto", "userpb", addr, user))), ".slogschema.pb.go")
	assertContains(t, src, "func (x *Address) MarshalSLog()")
	assertContains(t, src, "func (x *User) MarshalSLog()")
	assertContains(t, src, "MarshalSLog()...")
}

func TestWellKnownTimestamp(t *testing.T) {
	msg := &descriptorpb.DescriptorProto{
		Name:  proto.String("Event"),
		Field: []*descriptorpb.FieldDescriptorProto{msgField("created_at", ".google.protobuf.Timestamp", 1)},
	}
	src := getFile(t, generate(makeRequest(simpleFile("event.proto", "eventpb", msg))), ".slogschema.pb.go")
	assertContains(t, src, "AsTime()")
	assertContains(t, src, "slog.TimeValue")
}

func TestWellKnownDuration(t *testing.T) {
	msg := &descriptorpb.DescriptorProto{
		Name:  proto.String("Task"),
		Field: []*descriptorpb.FieldDescriptorProto{msgField("timeout", ".google.protobuf.Duration", 1)},
	}
	src := getFile(t, generate(makeRequest(simpleFile("task.proto", "taskpb", msg))), ".slogschema.pb.go")
	assertContains(t, src, "AsDuration()")
	assertContains(t, src, "slog.DurationValue")
}

func TestOneofFields(t *testing.T) {
	idx := int32(0)
	f1 := field("email", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1)
	f1.OneofIndex = &idx
	f2 := field("phone", descriptorpb.FieldDescriptorProto_TYPE_STRING, 2)
	f2.OneofIndex = &idx

	msg := &descriptorpb.DescriptorProto{
		Name:  proto.String("Contact"),
		Field: []*descriptorpb.FieldDescriptorProto{f1, f2},
		OneofDecl: []*descriptorpb.OneofDescriptorProto{
			{Name: proto.String("contact_method")},
		},
	}
	src := getFile(t, generate(makeRequest(simpleFile("contact.proto", "contactpb", msg))), ".slogschema.pb.go")
	assertContains(t, src, "ContactMethod")
	assertContains(t, src, "Contact_Email")
	assertContains(t, src, "Contact_Phone")
	assertContains(t, src, `"email"`)
	assertContains(t, src, `"phone"`)
}

func TestOneofSkipField(t *testing.T) {
	idx := int32(0)
	f1 := field("email", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1)
	f1.OneofIndex = &idx
	f2 := withSlogOpts(field("phone", descriptorpb.FieldDescriptorProto_TYPE_STRING, 2), "", false, false, true)
	f2.OneofIndex = &idx

	msg := &descriptorpb.DescriptorProto{
		Name:  proto.String("Contact"),
		Field: []*descriptorpb.FieldDescriptorProto{f1, f2},
		OneofDecl: []*descriptorpb.OneofDescriptorProto{
			{Name: proto.String("contact_method")},
		},
	}
	src := getFile(t, generate(makeRequest(simpleFile("contact.proto", "contactpb", msg))), ".slogschema.pb.go")
	assertContains(t, src, "Contact_Email")
	assertNotContains(t, src, "Contact_Phone")
}

func TestNestedMessageType(t *testing.T) {
	inner := &descriptorpb.DescriptorProto{
		Name:  proto.String("Config"),
		Field: []*descriptorpb.FieldDescriptorProto{field("value", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1)},
	}
	outer := &descriptorpb.DescriptorProto{
		Name:       proto.String("Service"),
		NestedType: []*descriptorpb.DescriptorProto{inner},
		Field:      []*descriptorpb.FieldDescriptorProto{msgField("config", ".Service.Config", 1)},
	}
	src := getFile(t, generate(makeRequest(simpleFile("svc.proto", "svcpb", outer))), ".slogschema.pb.go")
	assertContains(t, src, "func (x *Service_Config) MarshalSLog()")
	assertContains(t, src, "func (x *Service) MarshalSLog()")
}

// encodeMessageOpts encodes a slogschema MessageOptions extension onto a DescriptorProto.
func withDefaultSkip(msg *descriptorpb.DescriptorProto) *descriptorpb.DescriptorProto {
	// Encode inner MessageOptions{default_skip: true}
	var inner []byte
	inner = protowire.AppendTag(inner, 1, protowire.VarintType)
	inner = protowire.AppendVarint(inner, 1)

	// Wrap as extension field 1071 on MessageOptions.
	var ext []byte
	ext = protowire.AppendTag(ext, 77148, protowire.BytesType)
	ext = protowire.AppendBytes(ext, inner)

	if msg.Options == nil {
		msg.Options = &descriptorpb.MessageOptions{}
	}
	msg.Options.ProtoReflect().SetUnknown(ext)
	return msg
}

func TestDefaultSkipOmitsUnannotatedFields(t *testing.T) {
	msg := withDefaultSkip(&descriptorpb.DescriptorProto{
		Name: proto.String("Payment"),
		Field: []*descriptorpb.FieldDescriptorProto{
			field("amount", descriptorpb.FieldDescriptorProto_TYPE_INT64, 1),                                    // no annotation — skipped
			withSlogOpts(field("card_number", descriptorpb.FieldDescriptorProto_TYPE_STRING, 2), "", true, false, false), // annotated — included
			withSlogOpts(field("currency", descriptorpb.FieldDescriptorProto_TYPE_STRING, 3), "ccy", false, false, false), // annotated with name — included
		},
	})
	src := getFile(t, generate(makeRequest(simpleFile("payment.proto", "paymentpb", msg))), ".slogschema.pb.go")

	assertNotContains(t, src, "GetAmount()")  // unannotated — absent
	assertContains(t, src, "GetCardNumber()") // hashed annotation — present
	assertContains(t, src, "GetCurrency()")  // name annotation — present
	assertContains(t, src, `"ccy"`)
}

func TestDefaultSkipExplicitSkipStillSkips(t *testing.T) {
	msg := withDefaultSkip(&descriptorpb.DescriptorProto{
		Name: proto.String("User"),
		Field: []*descriptorpb.FieldDescriptorProto{
			withSlogOpts(field("email", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1), "", true, false, false),        // annotated — present
			withSlogOpts(field("password", descriptorpb.FieldDescriptorProto_TYPE_STRING, 2), "", false, false, true),     // skip:true — absent
		},
	})
	src := getFile(t, generate(makeRequest(simpleFile("user.proto", "userpb", msg))), ".slogschema.pb.go")

	assertContains(t, src, "GetEmail()")
	assertNotContains(t, src, "GetPassword()")
}

func TestDefaultSkipOneofUnannotatedSkipped(t *testing.T) {
	idx := int32(0)
	f1 := field("email", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1)
	f1.OneofIndex = &idx
	f2 := withSlogOpts(field("phone", descriptorpb.FieldDescriptorProto_TYPE_STRING, 2), "", false, false, false)
	f2.OneofIndex = &idx

	msg := withDefaultSkip(&descriptorpb.DescriptorProto{
		Name:  proto.String("Contact"),
		Field: []*descriptorpb.FieldDescriptorProto{f1, f2},
		OneofDecl: []*descriptorpb.OneofDescriptorProto{
			{Name: proto.String("contact_method")},
		},
	})
	src := getFile(t, generate(makeRequest(simpleFile("contact.proto", "contactpb", msg))), ".slogschema.pb.go")

	assertNotContains(t, src, "Contact_Email") // unannotated — absent
	assertContains(t, src, "Contact_Phone")    // annotated — present
}

func TestNoDefaultSkipIncludesAll(t *testing.T) {
	// Sanity check: without default_skip all unannotated fields are still included.
	msg := &descriptorpb.DescriptorProto{
		Name: proto.String("Order"),
		Field: []*descriptorpb.FieldDescriptorProto{
			field("id", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1),
			field("total", descriptorpb.FieldDescriptorProto_TYPE_INT64, 2),
		},
	}
	src := getFile(t, generate(makeRequest(simpleFile("order.proto", "orderpb", msg))), ".slogschema.pb.go")
	assertContains(t, src, "GetId()")
	assertContains(t, src, "GetTotal()")
}

func TestNoMessagesProducesNoOutput(t *testing.T) {
	resp := generate(makeRequest(simpleFile("empty.proto", "emptypb")))
	if len(resp.File) != 0 {
		t.Errorf("expected no output files for empty proto, got %v", fileNames(resp))
	}
}

func TestGoPackageSemicolon(t *testing.T) {
	syntax := "proto3"
	f := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("foo.proto"),
		Package: proto.String("foo"),
		Syntax:  &syntax,
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Bar"), Field: []*descriptorpb.FieldDescriptorProto{
				field("x", descriptorpb.FieldDescriptorProto_TYPE_STRING, 1),
			}},
		},
		Options: &descriptorpb.FileOptions{GoPackage: proto.String("github.com/example/foo;foo")},
	}
	src := getFile(t, generate(makeRequest(f)), ".slogschema.pb.go")
	assertContains(t, src, "package foo")
}
