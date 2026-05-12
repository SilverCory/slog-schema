// protoc-gen-slogschema is a protoc plugin that generates MarshalSLog() methods
// on existing protoc-gen-go message types, making them compatible with
// slog-schema's ToAttrs / ToAttrsAny without any separate logging struct.
//
// Usage:
//
//	protoc --slogschema_out=. path/to/file.proto
//
// Each input .proto file produces a corresponding _slogschema.pb.go file in
// the same package as the protoc-gen-go output, adding a MarshalSLog() method
// to each message type.
package main

import (
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	pluginpb "google.golang.org/protobuf/types/pluginpb"
)

const extensionFieldNumber protowire.Number = 77148

// fieldOptions holds the slogschema annotations for a proto field.
type fieldOptions struct {
	Name      string
	Hashed    bool
	Omitempty bool
	Skip      bool
}

// getFieldOptions extracts slogschema field annotations from raw FieldOptions unknown fields.
func getFieldOptions(opts *descriptorpb.FieldOptions) *fieldOptions {
	if opts == nil {
		return nil
	}
	raw := opts.ProtoReflect().GetUnknown()
	if len(raw) == 0 {
		return nil
	}
	var msgBytes []byte
	b := raw
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			break
		}
		b = b[n:]
		if num == extensionFieldNumber && typ == protowire.BytesType {
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				break
			}
			msgBytes = v
			b = b[n:]
		} else {
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				break
			}
			b = b[n:]
		}
	}
	if msgBytes == nil {
		return nil
	}
	o := &fieldOptions{}
	for len(msgBytes) > 0 {
		num, typ, n := protowire.ConsumeTag(msgBytes)
		if n < 0 {
			break
		}
		msgBytes = msgBytes[n:]
		switch {
		case num == 1 && typ == protowire.BytesType:
			v, n := protowire.ConsumeBytes(msgBytes)
			if n < 0 {
				return nil
			}
			o.Name = string(v)
			msgBytes = msgBytes[n:]
		case num == 2 && typ == protowire.VarintType:
			v, n := protowire.ConsumeVarint(msgBytes)
			if n < 0 {
				return nil
			}
			o.Hashed = v != 0
			msgBytes = msgBytes[n:]
		case num == 3 && typ == protowire.VarintType:
			v, n := protowire.ConsumeVarint(msgBytes)
			if n < 0 {
				return nil
			}
			o.Omitempty = v != 0
			msgBytes = msgBytes[n:]
		case num == 4 && typ == protowire.VarintType:
			v, n := protowire.ConsumeVarint(msgBytes)
			if n < 0 {
				return nil
			}
			o.Skip = v != 0
			msgBytes = msgBytes[n:]
		default:
			n := protowire.ConsumeFieldValue(num, typ, msgBytes)
			if n < 0 {
				return nil
			}
			msgBytes = msgBytes[n:]
		}
	}
	return o
}

// messageOptions holds the slogschema message-level annotation.
type messageOptions struct {
	DefaultSkip bool
}

// getMessageOptions extracts slogschema message annotations from raw MessageOptions unknown fields.
func getMessageOptions(opts *descriptorpb.MessageOptions) *messageOptions {
	if opts == nil {
		return nil
	}
	raw := opts.ProtoReflect().GetUnknown()
	if len(raw) == 0 {
		return nil
	}
	var msgBytes []byte
	b := raw
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			break
		}
		b = b[n:]
		if num == extensionFieldNumber && typ == protowire.BytesType {
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				break
			}
			msgBytes = v
			b = b[n:]
		} else {
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				break
			}
			b = b[n:]
		}
	}
	if msgBytes == nil {
		return nil
	}
	o := &messageOptions{}
	for len(msgBytes) > 0 {
		num, typ, n := protowire.ConsumeTag(msgBytes)
		if n < 0 {
			break
		}
		msgBytes = msgBytes[n:]
		if num == 1 && typ == protowire.VarintType {
			v, n := protowire.ConsumeVarint(msgBytes)
			if n < 0 {
				return nil
			}
			o.DefaultSkip = v != 0
			msgBytes = msgBytes[n:]
		} else {
			n := protowire.ConsumeFieldValue(num, typ, msgBytes)
			if n < 0 {
				return nil
			}
			msgBytes = msgBytes[n:]
		}
	}
	return o
}

// version is set at build time via -ldflags="-X main.version=v1.2.3".
var version = "dev"

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal("reading stdin: %v", err)
	}

	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(input, req); err != nil {
		fatal("unmarshalling request: %v", err)
	}

	resp := generate(req)

	out, err := proto.Marshal(resp)
	if err != nil {
		fatal("marshalling response: %v", err)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		fatal("writing response: %v", err)
	}
}

func generate(req *pluginpb.CodeGeneratorRequest) *pluginpb.CodeGeneratorResponse {
	resp := &pluginpb.CodeGeneratorResponse{
		SupportedFeatures: proto.Uint64(uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)),
	}

	toGenerate := make(map[string]bool, len(req.FileToGenerate))
	for _, name := range req.FileToGenerate {
		toGenerate[name] = true
	}

	for _, f := range req.ProtoFile {
		if !toGenerate[f.GetName()] {
			continue
		}
		content, err := generateFile(f)
		if err != nil {
			errStr := err.Error()
			resp.Error = &errStr
			return resp
		}
		if content == "" {
			continue
		}
		resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
			Name:    proto.String(outputFilename(f.GetName())),
			Content: proto.String(content),
		})
	}
	return resp
}

func generateFile(f *descriptorpb.FileDescriptorProto) (string, error) {
	if len(f.MessageType) == 0 {
		return "", nil
	}

	pkgName, importPath := goPackage(f)
	imports := newImportSet(importPath)
	imports.add("log/slog")

	var methods strings.Builder
	for _, msg := range f.MessageType {
		if err := writeMethod(&methods, imports, msg, ""); err != nil {
			return "", fmt.Errorf("%s: %w", f.GetName(), err)
		}
	}

	var b strings.Builder
	b.WriteString("// Code generated by protoc-gen-slogschema. DO NOT EDIT.\n")
	b.WriteString(fmt.Sprintf("// source: %s\n\n", f.GetName()))
	b.WriteString(fmt.Sprintf("package %s\n\n", pkgName))
	b.WriteString("import (\n")
	b.WriteString(imports.render())
	b.WriteString(")\n\n")
	b.WriteString(methods.String())

	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return b.String(), fmt.Errorf("formatting generated code: %w", err)
	}
	return string(src), nil
}

// writeMethod writes a MarshalSLog() method for a message type and recurses into nested types.
func writeMethod(b *strings.Builder, imports *importSet, msg *descriptorpb.DescriptorProto, prefix string) error {
	typeName := prefix + msg.GetName()

	// Recurse into nested types first.
	for _, nested := range msg.NestedType {
		if nested.GetOptions().GetMapEntry() {
			continue
		}
		if err := writeMethod(b, imports, nested, typeName+"_"); err != nil {
			return err
		}
	}

	// Group oneof field indices by oneof index.
	oneofFields := make([][]int, len(msg.OneofDecl))
	for i, f := range msg.Field {
		if f.OneofIndex != nil {
			oneofFields[f.GetOneofIndex()] = append(oneofFields[f.GetOneofIndex()], i)
		}
	}

	b.WriteString(fmt.Sprintf("func (x *%s) MarshalSLog() []slog.Attr {\n", typeName))
	b.WriteString("\tif x == nil {\n\t\treturn nil\n\t}\n")
	b.WriteString("\treturn []slog.Attr{\n")

	msgOpts := getMessageOptions(msg.Options)
	defaultSkip := msgOpts != nil && msgOpts.DefaultSkip

	writtenOneof := make(map[int32]bool)

	for _, field := range msg.Field {
		opts := getFieldOptions(field.Options)
		// Under default_skip, fields with no annotation are skipped.
		// An explicit skip:true also skips. An annotation (any field set) opts in.
		if opts != nil && opts.Skip {
			continue
		}
		if defaultSkip && opts == nil {
			continue
		}

		if field.OneofIndex != nil {
			idx := field.GetOneofIndex()
			if writtenOneof[idx] {
				continue
			}
			writtenOneof[idx] = true
			writeOneofAttr(b, imports, msg, defaultSkip, idx, oneofFields[idx])
			continue
		}

		writeFieldAttr(b, imports, field, opts)
	}

	b.WriteString("\t}\n}\n\n")
	return nil
}

// writeFieldAttr emits a single slog.Attr expression for a regular field.
func writeFieldAttr(b *strings.Builder, imports *importSet, field *descriptorpb.FieldDescriptorProto, opts *fieldOptions) {
	key := slogKey(field, opts)
	getter := "x.Get" + camelCase(field.GetName()) + "()"

	// repeated → group
	if field.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		imports.add("strconv")
		b.WriteString(fmt.Sprintf("\tfunc() slog.Attr {\n"))
		b.WriteString(fmt.Sprintf("\t\telems := %s\n", getter))
		b.WriteString(fmt.Sprintf("\t\tattrs := make([]slog.Attr, len(elems))\n"))
		b.WriteString(fmt.Sprintf("\t\tfor i, v := range elems {\n"))
		b.WriteString(fmt.Sprintf("\t\t\tattrs[i] = slog.Attr{Key: strconv.Itoa(i), Value: %s}\n", scalarValue(field, "v", imports)))
		b.WriteString(fmt.Sprintf("\t\t}\n"))
		b.WriteString(fmt.Sprintf("\t\treturn slog.Attr{Key: %q, Value: slog.GroupValue(attrs...)}\n", key))
		b.WriteString(fmt.Sprintf("\t}(),\n"))
		return
	}

	omit := opts != nil && opts.Omitempty
	hashed := opts != nil && opts.Hashed

	if omit {
		b.WriteString(fmt.Sprintf("\tfunc() slog.Attr {\n"))
		b.WriteString(fmt.Sprintf("\t\tif v := %s; !isEmpty(v) {\n", getter))
		if hashed {
			imports.add("crypto/sha256")
			imports.add("fmt")
			b.WriteString(fmt.Sprintf("\t\t\tsum := sha256.Sum256([]byte(fmt.Sprintf(\"%%v\", v)))\n"))
			b.WriteString(fmt.Sprintf("\t\t\treturn slog.String(%q, fmt.Sprintf(\"sha256:%%x\", sum))\n", key))
		} else {
			b.WriteString(fmt.Sprintf("\t\t\treturn slog.Attr{Key: %q, Value: %s}\n", key, scalarValue(field, "v", imports)))
		}
		b.WriteString(fmt.Sprintf("\t\t}\n\t\treturn slog.Attr{}\n\t}(),\n"))
		return
	}

	if hashed {
		imports.add("crypto/sha256")
		imports.add("fmt")
		b.WriteString(fmt.Sprintf("\tfunc() slog.Attr {\n"))
		b.WriteString(fmt.Sprintf("\t\tsum := sha256.Sum256([]byte(fmt.Sprintf(\"%%v\", %s)))\n", getter))
		b.WriteString(fmt.Sprintf("\t\treturn slog.String(%q, fmt.Sprintf(\"sha256:%%x\", sum))\n", key))
		b.WriteString(fmt.Sprintf("\t}(),\n"))
		return
	}

	b.WriteString(fmt.Sprintf("\t{Key: %q, Value: %s},\n", key, scalarValue(field, getter, imports)))
}

// writeOneofAttr emits a runtime type-switch for a oneof field group.
func writeOneofAttr(b *strings.Builder, imports *importSet, msg *descriptorpb.DescriptorProto, defaultSkip bool, _ int32, fieldIndices []int) {
	// We switch on the oneof wrapper interface returned by the getter for the
	// first variant's field name; protoc-gen-go names the accessor after the oneof.
	// We use a type switch on the concrete wrapper types.
	b.WriteString("\tfunc() slog.Attr {\n")
	b.WriteString(fmt.Sprintf("\t\tswitch v := x.%s.(type) {\n", oneofGetterName(msg, fieldIndices)))

	for _, fi := range fieldIndices {
		field := msg.Field[fi]
		opts := getFieldOptions(field.Options)
		if opts != nil && opts.Skip {
			continue
		}
		if defaultSkip && opts == nil {
			continue
		}
		key := slogKey(field, opts)
		wrapperType := camelCase(msg.GetName()) + "_" + camelCase(field.GetName())
		valExpr := scalarValue(field, "v."+camelCase(field.GetName()), imports)
		b.WriteString(fmt.Sprintf("\t\tcase *%s:\n", wrapperType))
		b.WriteString(fmt.Sprintf("\t\t\treturn slog.Attr{Key: %q, Value: %s}\n", key, valExpr))
	}

	b.WriteString("\t\t}\n\t\treturn slog.Attr{}\n\t}(),\n")
}

// oneofGetterName returns the Go field name for the oneof interface on the message struct.
// protoc-gen-go uses the oneof decl name converted to CamelCase.
func oneofGetterName(msg *descriptorpb.DescriptorProto, fieldIndices []int) string {
	// The oneof index is the same for all fields in the group.
	idx := msg.Field[fieldIndices[0]].GetOneofIndex()
	return camelCase(msg.OneofDecl[idx].GetName())
}

// scalarValue returns a slog.Value expression for a field given a Go value expression.
func scalarValue(field *descriptorpb.FieldDescriptorProto, val string, imports *importSet) string {
	if field.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		return messageValue(field.GetTypeName(), val, imports)
	}
	switch field.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return fmt.Sprintf("slog.StringValue(%s)", val)
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return fmt.Sprintf("slog.BoolValue(%s)", val)
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return fmt.Sprintf("slog.Float64Value(%s)", val)
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return fmt.Sprintf("slog.Float64Value(float64(%s))", val)
	case descriptorpb.FieldDescriptorProto_TYPE_INT64,
		descriptorpb.FieldDescriptorProto_TYPE_SINT64,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return fmt.Sprintf("slog.Int64Value(%s)", val)
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		return fmt.Sprintf("slog.Uint64Value(%s)", val)
	case descriptorpb.FieldDescriptorProto_TYPE_INT32,
		descriptorpb.FieldDescriptorProto_TYPE_SINT32,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED32,
		descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		return fmt.Sprintf("slog.Int64Value(int64(%s))", val)
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		return fmt.Sprintf("slog.Uint64Value(uint64(%s))", val)
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		imports.add("fmt")
		return fmt.Sprintf("slog.StringValue(fmt.Sprintf(\"%%x\", %s))", val)
	default:
		imports.add("fmt")
		return fmt.Sprintf("slog.AnyValue(%s)", val)
	}
}

// messageValue returns a slog.Value expression for a message-typed field.
func messageValue(typeName, val string, imports *importSet) string {
	typeName = strings.TrimPrefix(typeName, ".")
	switch typeName {
	case "google.protobuf.Timestamp":
		return fmt.Sprintf("slog.TimeValue(%s.AsTime())", val)
	case "google.protobuf.Duration":
		return fmt.Sprintf("slog.DurationValue(%s.AsDuration())", val)
	case "google.protobuf.StringValue":
		return fmt.Sprintf("slog.StringValue(%s.GetValue())", val)
	case "google.protobuf.Int32Value", "google.protobuf.Int64Value":
		return fmt.Sprintf("slog.Int64Value(int64(%s.GetValue()))", val)
	case "google.protobuf.UInt32Value", "google.protobuf.UInt64Value":
		return fmt.Sprintf("slog.Uint64Value(uint64(%s.GetValue()))", val)
	case "google.protobuf.FloatValue", "google.protobuf.DoubleValue":
		return fmt.Sprintf("slog.Float64Value(float64(%s.GetValue()))", val)
	case "google.protobuf.BoolValue":
		return fmt.Sprintf("slog.BoolValue(%s.GetValue())", val)
	case "google.protobuf.Any":
		imports.add("fmt")
		return fmt.Sprintf("slog.StringValue(fmt.Sprintf(\"%%v\", %s))", val)
	}
	// Local message type — it will also implement MarshalSLog, so emit as a group.
	return fmt.Sprintf("slog.GroupValue(%s.MarshalSLog()...)", val)
}

// isEmpty generates a Go boolean expression for checking zero values of a field's type.
// Used inline in omitempty closures.
func isEmpty(field *descriptorpb.FieldDescriptorProto) string {
	// For repeated fields this is handled at the call site via len check.
	switch field.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_STRING,
		descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return "len(v) == 0"
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "!v"
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		return "v == nil"
	default:
		return "v == 0"
	}
}

func slogKey(field *descriptorpb.FieldDescriptorProto, opts *fieldOptions) string {
	if opts != nil && opts.Name != "" {
		return opts.Name
	}
	return field.GetName()
}

func outputFilename(protoName string) string {
	base := strings.TrimSuffix(protoName, filepath.Ext(protoName))
	return base + ".slogschema.pb.go"
}

func goPackage(f *descriptorpb.FileDescriptorProto) (name, importPath string) {
	if goPkg := f.GetOptions().GetGoPackage(); goPkg != "" {
		if i := strings.LastIndex(goPkg, ";"); i >= 0 {
			return goPkg[i+1:], goPkg[:i]
		}
		parts := strings.Split(goPkg, "/")
		return parts[len(parts)-1], goPkg
	}
	pkg := strings.ReplaceAll(f.GetPackage(), ".", "_")
	return pkg, pkg
}

func camelCase(s string) string {
	var b strings.Builder
	upper := true
	for _, r := range s {
		if r == '_' {
			upper = true
			continue
		}
		if upper {
			b.WriteRune(unicode.ToUpper(r))
			upper = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type importSet struct {
	ownPath string
	paths   map[string]bool
}

func newImportSet(ownPath string) *importSet {
	return &importSet{ownPath: ownPath, paths: make(map[string]bool)}
}

func (s *importSet) add(path string) {
	if path != s.ownPath {
		s.paths[path] = true
	}
}

func (s *importSet) render() string {
	var b strings.Builder
	for p := range s.paths {
		b.WriteString(fmt.Sprintf("\t%q\n", p))
	}
	return b.String()
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "protoc-gen-slogschema: "+format+"\n", args...)
	os.Exit(1)
}
