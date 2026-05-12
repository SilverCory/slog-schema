package slogschema

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	typeTime      = reflect.TypeOf(time.Time{})
	typeDuration  = reflect.TypeOf(time.Duration(0))
	typeLogValuer = reflect.TypeOf((*slog.LogValuer)(nil)).Elem()
)

// SLogMarshaler is implemented by types that want to control their own slog representation.
type SLogMarshaler interface {
	MarshalSLog() []slog.Attr
}

// ToAttrs converts a struct to a slice of slog.Attr using `slog:"field_name"` struct tags.
// Fields without a slog tag are skipped. Use slog:"-" to explicitly skip a field.
// The ",omitempty" option skips fields whose values are zero.
// The ",hashed" option replaces the value with its SHA-256 hash.
// Embedded structs without a tag are inlined; their fields are merged into the parent.
// If the value implements SLogMarshaler, MarshalSLog() is called instead.
func ToAttrs(v any) []slog.Attr {
	if m, ok := v.(SLogMarshaler); ok {
		return m.MarshalSLog()
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}
	return structAttrs(rv)
}

// ToAttrsAny converts a struct to a []any suitable for spreading into variadic
// logger methods such as slog.Info or logger.Error.
func ToAttrsAny(v any) []any {
	attrs := ToAttrs(v)
	out := make([]any, len(attrs))
	for i, a := range attrs {
		out[i] = a
	}
	return out
}

// tagOptions holds the parsed slog struct tag.
type tagOptions struct {
	name      string
	omitempty bool
	hashed    bool
}

func parseTag(tag string) tagOptions {
	parts := strings.Split(tag, ",")
	opts := tagOptions{name: parts[0]}
	for _, opt := range parts[1:] {
		switch opt {
		case "omitempty":
			opts.omitempty = true
		case "hashed":
			opts.hashed = true
		}
	}
	return opts
}

func structAttrs(rv reflect.Value) []slog.Attr {
	if rv.CanAddr() {
		if m, ok := rv.Addr().Interface().(SLogMarshaler); ok {
			return m.MarshalSLog()
		}
	}
	if m, ok := rv.Interface().(SLogMarshaler); ok {
		return m.MarshalSLog()
	}

	rt := rv.Type()
	attrs := make([]slog.Attr, 0, rt.NumField())

	for i := range rt.NumField() {
		field := rt.Field(i)
		fv := rv.Field(i)

		// Inline anonymous (embedded) fields that carry no slog tag.
		if field.Anonymous {
			if _, ok := field.Tag.Lookup("slog"); !ok {
				if inner, ok := derefStruct(fv); ok {
					attrs = append(attrs, structAttrs(inner)...)
				}
				continue
			}
		}

		tag, ok := field.Tag.Lookup("slog")
		if !ok || tag == "-" {
			continue
		}

		opts := parseTag(tag)
		if opts.omitempty && isEmpty(fv) {
			continue
		}

		val := valueFor(fv)
		if opts.hashed {
			val = hashValue(fv)
		}

		attrs = append(attrs, slog.Attr{Key: opts.name, Value: val})
	}

	return attrs
}

func valueFor(v reflect.Value) slog.Value {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return slog.AnyValue(nil)
		}
		v = v.Elem()
	}

	t := v.Type()

	if t.Implements(typeLogValuer) {
		return v.Interface().(slog.LogValuer).LogValue()
	}
	if reflect.PointerTo(t).Implements(typeLogValuer) && v.CanAddr() {
		return v.Addr().Interface().(slog.LogValuer).LogValue()
	}

	switch t {
	case typeTime:
		return slog.TimeValue(v.Interface().(time.Time))
	case typeDuration:
		return slog.DurationValue(time.Duration(v.Int()))
	}

	switch v.Kind() {
	case reflect.String:
		return slog.StringValue(v.String())
	case reflect.Bool:
		return slog.BoolValue(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return slog.Int64Value(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return slog.Uint64Value(v.Uint())
	case reflect.Float32, reflect.Float64:
		return slog.Float64Value(v.Float())
	case reflect.Struct:
		return slog.GroupValue(structAttrs(v)...)
	case reflect.Map:
		return slog.GroupValue(mapAttrs(v)...)
	case reflect.Slice, reflect.Array:
		return slog.GroupValue(sliceAttrs(v)...)
	default:
		return slog.AnyValue(v.Interface())
	}
}

func mapAttrs(v reflect.Value) []slog.Attr {
	if v.IsNil() {
		return nil
	}
	attrs := make([]slog.Attr, 0, v.Len())
	for _, key := range v.MapKeys() {
		attrs = append(attrs, slog.Attr{
			Key:   fmt.Sprintf("%v", key.Interface()),
			Value: valueFor(v.MapIndex(key)),
		})
	}
	return attrs
}

func sliceAttrs(v reflect.Value) []slog.Attr {
	if v.Kind() == reflect.Slice && v.IsNil() {
		return nil
	}
	attrs := make([]slog.Attr, v.Len())
	for i := range v.Len() {
		attrs[i] = slog.Attr{
			Key:   strconv.Itoa(i),
			Value: valueFor(v.Index(i)),
		}
	}
	return attrs
}

// derefStruct dereferences a value to a struct, returning false if it is a nil pointer.
func derefStruct(v reflect.Value) (reflect.Value, bool) {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return v, false
		}
		v = v.Elem()
	}
	return v, v.Kind() == reflect.Struct
}

func isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map, reflect.Chan:
		return v.IsNil() || v.Len() == 0
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Struct:
		if v.Type() == typeTime {
			return v.Interface().(time.Time).IsZero()
		}
		return false
	default:
		return false
	}
}

func hashValue(v reflect.Value) slog.Value {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%v", v.Interface())))
	return slog.StringValue(fmt.Sprintf("sha256:%x", sum))
}
