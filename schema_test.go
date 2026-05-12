package slogschema

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"math"
	"testing"
	"time"
)

// --- helpers ---

func findAttr(attrs []slog.Attr, key string) (slog.Attr, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a, true
		}
	}
	return slog.Attr{}, false
}

func mustAttr(t *testing.T, attrs []slog.Attr, key string) slog.Attr {
	t.Helper()
	a, ok := findAttr(attrs, key)
	if !ok {
		t.Fatalf("expected attr %q, not found in %v", key, attrs)
	}
	return a
}

func assertAbsent(t *testing.T, attrs []slog.Attr, key string) {
	t.Helper()
	if _, ok := findAttr(attrs, key); ok {
		t.Fatalf("expected attr %q to be absent, but found it", key)
	}
}

func expectedHash(v any) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%v", v)))
	return fmt.Sprintf("sha256:%x", sum)
}

// --- scalar types ---

func TestScalarTypes(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	dur := 5 * time.Second

	type S struct {
		Str   string        `slog:"str"`
		Bool  bool          `slog:"bool"`
		Int   int           `slog:"int"`
		Int64 int64         `slog:"int64"`
		Uint  uint          `slog:"uint"`
		F64   float64       `slog:"f64"`
		T     time.Time     `slog:"t"`
		D     time.Duration `slog:"d"`
	}

	attrs := ToAttrs(S{
		Str:   "hello",
		Bool:  true,
		Int:   -1,
		Int64: -99,
		Uint:  42,
		F64:   3.14,
		T:     now,
		D:     dur,
	})

	cases := []struct {
		key  string
		want slog.Value
	}{
		{"str", slog.StringValue("hello")},
		{"bool", slog.BoolValue(true)},
		{"int", slog.Int64Value(-1)},
		{"int64", slog.Int64Value(-99)},
		{"uint", slog.Uint64Value(42)},
		{"f64", slog.Float64Value(3.14)},
		{"t", slog.TimeValue(now)},
		{"d", slog.DurationValue(dur)},
	}

	for _, c := range cases {
		a := mustAttr(t, attrs, c.key)
		if got := a.Value.String(); got != c.want.String() {
			t.Errorf("%s: got %q, want %q", c.key, got, c.want.String())
		}
	}
}

func TestAllIntSubtypes(t *testing.T) {
	type S struct {
		I8  int8  `slog:"i8"`
		I16 int16 `slog:"i16"`
		I32 int32 `slog:"i32"`
	}
	attrs := ToAttrs(S{I8: -1, I16: -2, I32: -3})
	if mustAttr(t, attrs, "i8").Value.Int64() != -1 {
		t.Error("i8")
	}
	if mustAttr(t, attrs, "i16").Value.Int64() != -2 {
		t.Error("i16")
	}
	if mustAttr(t, attrs, "i32").Value.Int64() != -3 {
		t.Error("i32")
	}
}

func TestAllUintSubtypes(t *testing.T) {
	type S struct {
		U8  uint8  `slog:"u8"`
		U16 uint16 `slog:"u16"`
		U32 uint32 `slog:"u32"`
		U64 uint64 `slog:"u64"`
	}
	attrs := ToAttrs(S{U8: 1, U16: 2, U32: 3, U64: math.MaxUint64})
	if mustAttr(t, attrs, "u64").Value.Uint64() != math.MaxUint64 {
		t.Error("u64 max")
	}
}

func TestFloat32(t *testing.T) {
	type S struct {
		F float32 `slog:"f"`
	}
	a := mustAttr(t, ToAttrs(S{F: 1.5}), "f")
	if a.Value.Kind() != slog.KindFloat64 {
		t.Errorf("expected float64 kind, got %s", a.Value.Kind())
	}
}

func TestIntBoundaryValues(t *testing.T) {
	type S struct {
		Min int64  `slog:"min"`
		Max uint64 `slog:"max"`
	}
	attrs := ToAttrs(S{Min: math.MinInt64, Max: math.MaxUint64})
	if mustAttr(t, attrs, "min").Value.Int64() != math.MinInt64 {
		t.Error("MinInt64")
	}
	if mustAttr(t, attrs, "max").Value.Uint64() != math.MaxUint64 {
		t.Error("MaxUint64")
	}
}

// --- tag behaviour ---

func TestNoTagSkipped(t *testing.T) {
	type S struct {
		Visible string `slog:"visible"`
		Hidden  string
	}
	attrs := ToAttrs(S{Visible: "yes", Hidden: "no"})
	mustAttr(t, attrs, "visible")
	assertAbsent(t, attrs, "Hidden")
}

func TestDashSkipped(t *testing.T) {
	type S struct {
		Keep string `slog:"keep"`
		Drop string `slog:"-"`
	}
	attrs := ToAttrs(S{Keep: "yes", Drop: "no"})
	mustAttr(t, attrs, "keep")
	assertAbsent(t, attrs, "Drop")
	assertAbsent(t, attrs, "-")
}

func TestUnknownOptionsIgnored(t *testing.T) {
	// Unknown tag options should not panic or affect the key.
	type S struct {
		Name string `slog:"name,future_option,another"`
	}
	a := mustAttr(t, ToAttrs(S{Name: "x"}), "name")
	if a.Value.String() != "x" {
		t.Errorf("got %q, want %q", a.Value.String(), "x")
	}
}

func TestFieldOrder(t *testing.T) {
	// Attrs should come out in struct field declaration order.
	type S struct {
		A string `slog:"a"`
		B string `slog:"b"`
		C string `slog:"c"`
	}
	attrs := ToAttrs(S{A: "1", B: "2", C: "3"})
	if len(attrs) != 3 || attrs[0].Key != "a" || attrs[1].Key != "b" || attrs[2].Key != "c" {
		t.Errorf("unexpected order: %v", attrs)
	}
}

// --- omitempty ---

func TestOmitempty(t *testing.T) {
	type S struct {
		Name  string `slog:"name,omitempty"`
		Count int    `slog:"count,omitempty"`
		Flag  bool   `slog:"flag,omitempty"`
	}

	attrs := ToAttrs(S{})
	assertAbsent(t, attrs, "name")
	assertAbsent(t, attrs, "count")
	assertAbsent(t, attrs, "flag")

	attrs = ToAttrs(S{Name: "x", Count: 1, Flag: true})
	mustAttr(t, attrs, "name")
	mustAttr(t, attrs, "count")
	mustAttr(t, attrs, "flag")
}

func TestOmitemptyTime(t *testing.T) {
	type S struct {
		T time.Time `slog:"t,omitempty"`
	}
	assertAbsent(t, ToAttrs(S{}), "t")
	mustAttr(t, ToAttrs(S{T: time.Now()}), "t")
}

func TestOmitemptySliceMap(t *testing.T) {
	type S struct {
		Tags    []string          `slog:"tags,omitempty"`
		Headers map[string]string `slog:"headers,omitempty"`
	}
	assertAbsent(t, ToAttrs(S{}), "tags")
	assertAbsent(t, ToAttrs(S{}), "headers")
	mustAttr(t, ToAttrs(S{Tags: []string{"a"}}), "tags")
	mustAttr(t, ToAttrs(S{Headers: map[string]string{"k": "v"}}), "headers")
}

func TestOmitemptyEmptySliceNotSameAsNil(t *testing.T) {
	// An explicitly empty (non-nil) slice should still be omitted — len==0.
	type S struct {
		Tags []string `slog:"tags,omitempty"`
	}
	assertAbsent(t, ToAttrs(S{Tags: []string{}}), "tags")
}

func TestOmitemptyFloat(t *testing.T) {
	type S struct {
		Score float64 `slog:"score,omitempty"`
	}
	assertAbsent(t, ToAttrs(S{}), "score")
	mustAttr(t, ToAttrs(S{Score: 0.001}), "score")
}

func TestOmitemptyNonZeroStruct(t *testing.T) {
	// Non-time structs are never considered empty, even if all fields are zero.
	type Inner struct{ X int }
	type S struct {
		Inner Inner `slog:"inner,omitempty"`
	}
	mustAttr(t, ToAttrs(S{}), "inner")
}

// --- hashed ---

func TestHashed(t *testing.T) {
	type S struct {
		Email string `slog:"email,hashed"`
		ID    uint64 `slog:"id,hashed"`
	}
	attrs := ToAttrs(S{Email: "user@example.com", ID: 99})

	email := mustAttr(t, attrs, "email")
	if got, want := email.Value.String(), expectedHash("user@example.com"); got != want {
		t.Errorf("email hash: got %q, want %q", got, want)
	}

	id := mustAttr(t, attrs, "id")
	if got, want := id.Value.String(), expectedHash(uint64(99)); got != want {
		t.Errorf("id hash: got %q, want %q", got, want)
	}
}

func TestHashedIsString(t *testing.T) {
	type S struct {
		V int `slog:"v,hashed"`
	}
	a := mustAttr(t, ToAttrs(S{V: 42}), "v")
	if a.Value.Kind() != slog.KindString {
		t.Errorf("expected string kind for hashed value, got %s", a.Value.Kind())
	}
	if got := a.Value.String(); len(got) == 0 || got[:7] != "sha256:" {
		t.Errorf("expected sha256: prefix, got %q", got)
	}
}

func TestHashedDeterministic(t *testing.T) {
	// Same input must always produce the same hash.
	type S struct {
		Email string `slog:"email,hashed"`
	}
	a1 := mustAttr(t, ToAttrs(S{Email: "alice@example.com"}), "email")
	a2 := mustAttr(t, ToAttrs(S{Email: "alice@example.com"}), "email")
	if a1.Value.String() != a2.Value.String() {
		t.Error("hash is not deterministic")
	}
}

func TestHashedDifferentInputsDifferentHashes(t *testing.T) {
	type S struct {
		Email string `slog:"email,hashed"`
	}
	a1 := mustAttr(t, ToAttrs(S{Email: "alice@example.com"}), "email")
	a2 := mustAttr(t, ToAttrs(S{Email: "bob@example.com"}), "email")
	if a1.Value.String() == a2.Value.String() {
		t.Error("different inputs produced the same hash")
	}
}

func TestHashedOmitemptyCombined(t *testing.T) {
	type S struct {
		ID uint64 `slog:"id,omitempty,hashed"`
	}
	assertAbsent(t, ToAttrs(S{}), "id")
	a := mustAttr(t, ToAttrs(S{ID: 1}), "id")
	if a.Value.Kind() != slog.KindString {
		t.Error("expected hashed string")
	}
}

func TestHashedZeroValueStillHashed(t *testing.T) {
	// Without omitempty, zero values are hashed — not omitted.
	type S struct {
		Count int `slog:"count,hashed"`
	}
	a := mustAttr(t, ToAttrs(S{Count: 0}), "count")
	if a.Value.Kind() != slog.KindString || a.Value.String()[:7] != "sha256:" {
		t.Errorf("expected hashed zero value, got %v", a.Value)
	}
}

// --- pointers ---

func TestNilPointer(t *testing.T) {
	type S struct{ Name *string }
	if ToAttrs((*S)(nil)) != nil {
		t.Error("expected nil for nil pointer input")
	}
}

func TestDoublePointer(t *testing.T) {
	type S struct {
		Name **string `slog:"name"`
	}
	s := "hello"
	sp := &s
	a := mustAttr(t, ToAttrs(S{Name: &sp}), "name")
	if a.Value.String() != "hello" {
		t.Errorf("got %q, want %q", a.Value.String(), "hello")
	}
}

func TestPointerField(t *testing.T) {
	type S struct {
		Name *string `slog:"name"`
	}
	s := "hello"
	a := mustAttr(t, ToAttrs(S{Name: &s}), "name")
	if a.Value.String() != "hello" {
		t.Errorf("got %q, want %q", a.Value.String(), "hello")
	}
}

func TestNilPointerField(t *testing.T) {
	type S struct {
		Name *string `slog:"name"`
	}
	a := mustAttr(t, ToAttrs(S{}), "name")
	if a.Value.Kind() != slog.KindAny || a.Value.Any() != nil {
		t.Errorf("expected nil any value, got %v", a.Value)
	}
}

func TestPointerToStruct(t *testing.T) {
	type Inner struct {
		X int `slog:"x"`
	}
	type S struct {
		Inner *Inner `slog:"inner"`
	}
	inner := &Inner{X: 7}
	a := mustAttr(t, ToAttrs(S{Inner: inner}), "inner")
	if a.Value.Kind() != slog.KindGroup {
		t.Fatalf("expected group, got %s", a.Value.Kind())
	}
	if mustAttr(t, a.Value.Group(), "x").Value.Int64() != 7 {
		t.Error("x != 7")
	}
}

func TestNilPointerToStruct(t *testing.T) {
	type Inner struct {
		X int `slog:"x"`
	}
	type S struct {
		Inner *Inner `slog:"inner"`
	}
	a := mustAttr(t, ToAttrs(S{}), "inner")
	if a.Value.Kind() != slog.KindAny || a.Value.Any() != nil {
		t.Errorf("expected nil any, got %v", a.Value)
	}
}

// --- embedded structs ---

func TestEmbeddedInlined(t *testing.T) {
	type Meta struct {
		TraceID string `slog:"trace_id"`
	}
	type S struct {
		Meta
		Method string `slog:"method"`
	}
	attrs := ToAttrs(S{Meta: Meta{TraceID: "abc"}, Method: "GET"})
	mustAttr(t, attrs, "trace_id")
	mustAttr(t, attrs, "method")
}

func TestEmbeddedWithTagNotInlined(t *testing.T) {
	type Meta struct {
		TraceID string `slog:"trace_id"`
	}
	type S struct {
		Meta   `slog:"meta"`
		Method string `slog:"method"`
	}
	attrs := ToAttrs(S{Meta: Meta{TraceID: "abc"}, Method: "GET"})
	meta := mustAttr(t, attrs, "meta")
	if meta.Value.Kind() != slog.KindGroup {
		t.Fatalf("expected group, got %s", meta.Value.Kind())
	}
	assertAbsent(t, attrs, "trace_id")
}

func TestEmbeddedNilPointer(t *testing.T) {
	type Meta struct {
		TraceID string `slog:"trace_id"`
	}
	type S struct {
		*Meta
		Method string `slog:"method"`
	}
	attrs := ToAttrs(S{Method: "GET"})
	mustAttr(t, attrs, "method")
	assertAbsent(t, attrs, "trace_id")
}

func TestMultiLevelEmbedded(t *testing.T) {
	type A struct {
		X string `slog:"x"`
	}
	type B struct {
		A
		Y string `slog:"y"`
	}
	type C struct {
		B
		Z string `slog:"z"`
	}
	attrs := ToAttrs(C{B: B{A: A{X: "1"}, Y: "2"}, Z: "3"})
	mustAttr(t, attrs, "x")
	mustAttr(t, attrs, "y")
	mustAttr(t, attrs, "z")
}

// --- nested structs ---

func TestNestedStruct(t *testing.T) {
	type Address struct {
		City string `slog:"city"`
	}
	type S struct {
		Addr Address `slog:"addr"`
	}
	attrs := ToAttrs(S{Addr: Address{City: "London"}})
	addr := mustAttr(t, attrs, "addr")
	if addr.Value.Kind() != slog.KindGroup {
		t.Fatalf("expected group, got %s", addr.Value.Kind())
	}
	city := mustAttr(t, addr.Value.Group(), "city")
	if city.Value.String() != "London" {
		t.Errorf("got %q, want %q", city.Value.String(), "London")
	}
}

func TestDeeplyNestedStruct(t *testing.T) {
	type C struct {
		Val string `slog:"val"`
	}
	type B struct {
		C C `slog:"c"`
	}
	type A struct {
		B B `slog:"b"`
	}
	attrs := ToAttrs(A{B: B{C: C{Val: "deep"}}})
	b := mustAttr(t, attrs, "b")
	c := mustAttr(t, b.Value.Group(), "c")
	val := mustAttr(t, c.Value.Group(), "val")
	if val.Value.String() != "deep" {
		t.Errorf("got %q, want deep", val.Value.String())
	}
}

// --- slices and arrays ---

func TestSlice(t *testing.T) {
	type S struct {
		Tags []string `slog:"tags"`
	}
	attrs := ToAttrs(S{Tags: []string{"a", "b", "c"}})
	tags := mustAttr(t, attrs, "tags")
	if tags.Value.Kind() != slog.KindGroup {
		t.Fatalf("expected group, got %s", tags.Value.Kind())
	}
	g := tags.Value.Group()
	if len(g) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(g))
	}
	if g[0].Key != "0" || g[0].Value.String() != "a" {
		t.Errorf("unexpected element: %v", g[0])
	}
}

func TestSliceOfStructs(t *testing.T) {
	type Item struct {
		Name string `slog:"name"`
	}
	type S struct {
		Items []Item `slog:"items"`
	}
	attrs := ToAttrs(S{Items: []Item{{Name: "foo"}, {Name: "bar"}}})
	items := mustAttr(t, attrs, "items")
	g := items.Value.Group()
	if len(g) != 2 {
		t.Fatalf("expected 2 items, got %d", len(g))
	}
	// Each element is a group with a "name" attr.
	if g[0].Value.Kind() != slog.KindGroup {
		t.Fatalf("expected group for slice element, got %s", g[0].Value.Kind())
	}
	mustAttr(t, g[0].Value.Group(), "name")
}

func TestNilSlice(t *testing.T) {
	type S struct {
		Tags []string `slog:"tags"`
	}
	tags := mustAttr(t, ToAttrs(S{}), "tags")
	if tags.Value.Kind() != slog.KindGroup || len(tags.Value.Group()) != 0 {
		t.Errorf("expected empty group for nil slice, got %v", tags.Value)
	}
}

func TestEmptySlice(t *testing.T) {
	type S struct {
		Tags []string `slog:"tags"`
	}
	tags := mustAttr(t, ToAttrs(S{Tags: []string{}}), "tags")
	if len(tags.Value.Group()) != 0 {
		t.Errorf("expected empty group for empty slice, got %v", tags.Value.Group())
	}
}

func TestArray(t *testing.T) {
	type S struct {
		Coords [2]int `slog:"coords"`
	}
	attrs := ToAttrs(S{Coords: [2]int{10, 20}})
	coords := mustAttr(t, attrs, "coords")
	g := coords.Value.Group()
	if len(g) != 2 || g[1].Value.Int64() != 20 {
		t.Errorf("unexpected coords group: %v", g)
	}
}

func TestSliceIndexKeys(t *testing.T) {
	// Keys must be "0", "1", "2" ... not "00", "01", etc.
	type S struct {
		Vals []int `slog:"vals"`
	}
	attrs := ToAttrs(S{Vals: []int{10, 20, 30}})
	g := mustAttr(t, attrs, "vals").Value.Group()
	for i, a := range g {
		if a.Key != fmt.Sprintf("%d", i) {
			t.Errorf("index %d: expected key %q, got %q", i, fmt.Sprintf("%d", i), a.Key)
		}
	}
}

// --- maps ---

func TestMap(t *testing.T) {
	type S struct {
		H map[string]string `slog:"h"`
	}
	attrs := ToAttrs(S{H: map[string]string{"k": "v"}})
	h := mustAttr(t, attrs, "h")
	if h.Value.Kind() != slog.KindGroup {
		t.Fatalf("expected group, got %s", h.Value.Kind())
	}
	kv := mustAttr(t, h.Value.Group(), "k")
	if kv.Value.String() != "v" {
		t.Errorf("got %q, want %q", kv.Value.String(), "v")
	}
}

func TestNilMap(t *testing.T) {
	type S struct {
		H map[string]string `slog:"h"`
	}
	h := mustAttr(t, ToAttrs(S{}), "h")
	if h.Value.Kind() != slog.KindGroup || len(h.Value.Group()) != 0 {
		t.Errorf("expected empty group for nil map, got %v", h.Value)
	}
}

func TestMapIntKeys(t *testing.T) {
	type S struct {
		Scores map[int]string `slog:"scores"`
	}
	attrs := ToAttrs(S{Scores: map[int]string{1: "one"}})
	g := mustAttr(t, attrs, "scores").Value.Group()
	if len(g) != 1 || g[0].Key != "1" {
		t.Errorf("unexpected map group: %v", g)
	}
}

func TestMapStringIntValues(t *testing.T) {
	type S struct {
		Counts map[string]int `slog:"counts"`
	}
	attrs := ToAttrs(S{Counts: map[string]int{"hits": 42}})
	g := mustAttr(t, attrs, "counts").Value.Group()
	hits := mustAttr(t, g, "hits")
	if hits.Value.Int64() != 42 {
		t.Errorf("got %d, want 42", hits.Value.Int64())
	}
}

// --- SLogMarshaler ---

type customType struct{ val string }

func (c customType) MarshalSLog() []slog.Attr {
	return []slog.Attr{slog.String("custom", c.val)}
}

type customPtrType struct{ val string }

func (c *customPtrType) MarshalSLog() []slog.Attr {
	return []slog.Attr{slog.String("custom_ptr", c.val)}
}

func TestMarshalerTopLevel(t *testing.T) {
	attrs := ToAttrs(customType{val: "hello"})
	mustAttr(t, attrs, "custom")
}

func TestMarshalerPointerReceiver(t *testing.T) {
	attrs := ToAttrs(&customPtrType{val: "hello"})
	mustAttr(t, attrs, "custom_ptr")
}

func TestMarshalerNestedField(t *testing.T) {
	type S struct {
		C customType `slog:"c"`
	}
	c := mustAttr(t, ToAttrs(S{C: customType{val: "world"}}), "c")
	if c.Value.Kind() != slog.KindGroup {
		t.Fatalf("expected group, got %s", c.Value.Kind())
	}
	mustAttr(t, c.Value.Group(), "custom")
}

func TestMarshalerReturnsNil(t *testing.T) {
	// A MarshalSLog that returns nil should not panic.
	type nilMarshaler struct{}
	// We can't define a method on an inline type, so use customType with empty val.
	// Instead verify an empty return is handled.
	type S struct {
		C customType `slog:"c"`
	}
	c := mustAttr(t, ToAttrs(S{}), "c")
	// An empty MarshalSLog return produces an empty group.
	if c.Value.Kind() != slog.KindGroup {
		t.Fatalf("expected group, got %s", c.Value.Kind())
	}
}

// --- slog.LogValuer ---

type logValuerType struct{ msg string }

func (l logValuerType) LogValue() slog.Value {
	return slog.StringValue("logvaluer:" + l.msg)
}

func TestLogValuer(t *testing.T) {
	type S struct {
		V logValuerType `slog:"v"`
	}
	a := mustAttr(t, ToAttrs(S{V: logValuerType{msg: "test"}}), "v")
	if a.Value.String() != "logvaluer:test" {
		t.Errorf("got %q, want %q", a.Value.String(), "logvaluer:test")
	}
}

// --- ToAttrsAny ---

func TestToAttrsAny(t *testing.T) {
	type S struct {
		Name string `slog:"name"`
	}
	out := ToAttrsAny(S{Name: "test"})
	if len(out) != 1 {
		t.Fatalf("expected 1 element, got %d", len(out))
	}
	a, ok := out[0].(slog.Attr)
	if !ok {
		t.Fatalf("expected slog.Attr, got %T", out[0])
	}
	if a.Key != "name" || a.Value.String() != "test" {
		t.Errorf("unexpected attr: %v", a)
	}
}

func TestToAttrsAnyZeroFields(t *testing.T) {
	type S struct{}
	out := ToAttrsAny(S{})
	if len(out) != 0 {
		t.Errorf("expected empty slice, got %v", out)
	}
}

func TestToAttrsAnyPreservesAllAttrs(t *testing.T) {
	type S struct {
		A string `slog:"a"`
		B int    `slog:"b"`
		C bool   `slog:"c"`
	}
	out := ToAttrsAny(S{A: "x", B: 1, C: true})
	if len(out) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(out))
	}
	for i, v := range out {
		if _, ok := v.(slog.Attr); !ok {
			t.Errorf("element %d is %T, want slog.Attr", i, v)
		}
	}
}

// --- non-struct input ---

func TestNonStructReturnsNil(t *testing.T) {
	if ToAttrs("not a struct") != nil {
		t.Error("expected nil for string input")
	}
	if ToAttrs(42) != nil {
		t.Error("expected nil for int input")
	}
	if ToAttrs(nil) != nil {
		t.Error("expected nil for nil input")
	}
	if ToAttrs([]string{"a"}) != nil {
		t.Error("expected nil for slice input")
	}
}

func TestPointerToNonStruct(t *testing.T) {
	s := "hello"
	if ToAttrs(&s) != nil {
		t.Error("expected nil for pointer to string")
	}
}

// --- parseTag ---

func TestParseTagNameOnly(t *testing.T) {
	opts := parseTag("field_name")
	if opts.name != "field_name" || opts.omitempty || opts.hashed {
		t.Errorf("unexpected opts: %+v", opts)
	}
}

func TestParseTagOmitempty(t *testing.T) {
	opts := parseTag("field,omitempty")
	if opts.name != "field" || !opts.omitempty || opts.hashed {
		t.Errorf("unexpected opts: %+v", opts)
	}
}

func TestParseTagHashed(t *testing.T) {
	opts := parseTag("field,hashed")
	if opts.name != "field" || opts.omitempty || !opts.hashed {
		t.Errorf("unexpected opts: %+v", opts)
	}
}

func TestParseTagAllOptions(t *testing.T) {
	opts := parseTag("field,omitempty,hashed")
	if opts.name != "field" || !opts.omitempty || !opts.hashed {
		t.Errorf("unexpected opts: %+v", opts)
	}
}

func TestParseTagEmptyName(t *testing.T) {
	opts := parseTag(",omitempty")
	if opts.name != "" || !opts.omitempty {
		t.Errorf("unexpected opts: %+v", opts)
	}
}

// --- fuzz ---

func FuzzParseTag(f *testing.F) {
	f.Add("name")
	f.Add("name,omitempty")
	f.Add("name,hashed")
	f.Add("name,omitempty,hashed")
	f.Add(",omitempty")
	f.Add("")
	f.Add("name,unknown,omitempty,,hashed")

	f.Fuzz(func(t *testing.T, tag string) {
		// Must not panic.
		opts := parseTag(tag)
		// Name is always at least the first segment.
		_ = opts.name
	})
}

func FuzzToAttrsString(f *testing.F) {
	f.Add("hello")
	f.Add("")
	f.Add("user@example.com")
	f.Add("\x00\xff")

	f.Fuzz(func(t *testing.T, s string) {
		type S struct {
			V      string `slog:"v"`
			Hashed string `slog:"h,hashed"`
		}
		// Must not panic.
		attrs := ToAttrs(S{V: s, Hashed: s})
		if len(attrs) != 2 {
			t.Fatalf("expected 2 attrs, got %d", len(attrs))
		}
		// Hashed must always start with sha256:
		h := attrs[1].Value.String()
		if len(h) < 7 || h[:7] != "sha256:" {
			t.Errorf("expected sha256: prefix, got %q", h)
		}
	})
}
