package deep_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/chaelub/deep"
	"reflect"
	"testing"
	"time"
)

func TestString(t *testing.T) {
	diff, got := deep.CompareS("foo", "foo")
	if got {
		t.Error("should be equal:", diff)
	}

	diff, got = deep.CompareS("foo", "bar")
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "foo != bar" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestFloat(t *testing.T) {
	diff, got := deep.CompareS(1.1, 1.1)
	if got {
		t.Error("should be equal:", diff)
	}

	diff, got = deep.CompareS(1.1234561, 1.1234562)
	if diff == nil {
		t.Error("no diff")
	}

	defaultFloatPrecision := deep.DefaultOptions.FloatPrecision
	defer func() { deep.DefaultOptions.FloatPrecision = defaultFloatPrecision }()
	deep.DefaultOptions.FloatPrecision = 6

	diff, got = deep.CompareS(1.1234561, 1.1234562)
	if got {
		t.Error("should be equal:", diff)
	}

	diff, got = deep.CompareS(1.123456, 1.123457)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "1.123456 != 1.123457" {
		t.Error("wrong diff:", diff[0])
	}

}

func TestInt(t *testing.T) {
	diff, _ := deep.CompareS(1, 1)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, _ = deep.CompareS(1, 2)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "1 != 2" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestUint(t *testing.T) {
	diff, got := deep.CompareS(uint(2), uint(2))
	if got {
		t.Error("should be equal:", diff)
	}

	diff, got = deep.CompareS(uint(2), uint(3))
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "2 != 3" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestBool(t *testing.T) {
	diff, got := deep.CompareS(true, true)
	if got {
		t.Error("should be equal:", diff)
	}

	diff, got = deep.CompareS(false, false)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, got = deep.CompareS(true, false)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "true != false" { // unless you're fipar
		t.Error("wrong diff:", diff[0])
	}
}

func TestTypeMismatch(t *testing.T) {
	type T1 int // same type kind (int)
	type T2 int // but different type
	var t1 T1 = 1
	var t2 T2 = 1
	diff, _ := deep.CompareS(t1, t2)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "deep_test.T1 != deep_test.T2" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestKindMismatch(t *testing.T) {
	deep.DefaultOptions.LogErrors = true

	var x int = 100
	var y float64 = 100
	diff, got := deep.CompareS(x, y)
	if !got {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "int != float64" {
		t.Error("wrong diff:", diff[0])
	}

	deep.DefaultOptions.LogErrors = false
}

func TestDeepRecursion(t *testing.T) {
	deep.DefaultOptions.MaxDepth = 2
	defer func() { deep.DefaultOptions.MaxDepth = 10 }()

	type s3 struct {
		S int
	}
	type s2 struct {
		S s3
	}
	type s1 struct {
		S s2
	}
	foo := map[string]s1{
		"foo": { // 1
			S: s2{ // 2
				S: s3{ // 3
					S: 42, // 4
				},
			},
		},
	}
	bar := map[string]s1{
		"foo": {
			S: s2{
				S: s3{
					S: 100,
				},
			},
		},
	}
	diff, _ := deep.CompareS(foo, bar)

	defaultMaxDepth := deep.DefaultOptions.MaxDepth
	deep.DefaultOptions.MaxDepth = 4
	defer func() { deep.DefaultOptions.MaxDepth = defaultMaxDepth }()

	diff, _ = deep.CompareS(foo, bar)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "foo.S.S.S: 42 != 100" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestMaxDiff(t *testing.T) {
	a := []int{1, 2, 3, 4, 5, 6, 7}
	b := []int{0, 0, 0, 0, 0, 0, 0}

	defaultMaxDiff := deep.DefaultOptions.MaxDiff
	deep.DefaultOptions.MaxDiff = 3
	defer func() { deep.DefaultOptions.MaxDiff = defaultMaxDiff }()

	diff, _ := deep.CompareS(a, b)
	if diff == nil {
		t.Fatal("no diffs")
	}
	if len(diff) != deep.DefaultOptions.MaxDiff {
		t.Errorf("got %d diffs, expected %d", len(diff), deep.DefaultOptions.MaxDiff)
	}

	defaultCompareUnexportedFields := deep.DefaultOptions.CompareUnexportedFields
	deep.DefaultOptions.CompareUnexportedFields = true
	defer func() { deep.DefaultOptions.CompareUnexportedFields = defaultCompareUnexportedFields }()
	type fiveFields struct {
		a int // unexported fields require ^
		b int
		c int
		d int
		e int
	}
	t1 := fiveFields{1, 2, 3, 4, 5}
	t2 := fiveFields{0, 0, 0, 0, 0}
	diff, _ = deep.CompareS(t1, t2)
	if diff == nil {
		t.Fatal("no diffs")
	}
	if len(diff) != deep.DefaultOptions.MaxDiff {
		t.Errorf("got %d diffs, expected %d", len(diff), deep.DefaultOptions.MaxDiff)
	}

	// Same keys, too many diffs
	m1 := map[int]int{
		1: 1,
		2: 2,
		3: 3,
		4: 4,
		5: 5,
	}
	m2 := map[int]int{
		1: 0,
		2: 0,
		3: 0,
		4: 0,
		5: 0,
	}
	diff, _ = deep.CompareS(m1, m2)
	if diff == nil {
		t.Fatal("no diffs")
	}
	if len(diff) != deep.DefaultOptions.MaxDiff {
		t.Log(diff)
		t.Errorf("got %d diffs, expected %d", len(diff), deep.DefaultOptions.MaxDiff)
	}

	// Too many missing keys
	m1 = map[int]int{
		1: 1,
		2: 2,
	}
	m2 = map[int]int{
		1: 1,
		2: 2,
		3: 0,
		4: 0,
		5: 0,
		6: 0,
		7: 0,
	}
	diff, _ = deep.CompareS(m1, m2)
	if diff == nil {
		t.Fatal("no diffs")
	}
	if len(diff) != deep.DefaultOptions.MaxDiff {
		t.Log(diff)
		t.Errorf("got %d diffs, expected %d", len(diff), deep.DefaultOptions.MaxDiff)
	}
}

func TestNotHandled(t *testing.T) {
	a := func(int) {}
	b := func(int) {}
	diff, _ := deep.CompareS(a, b)
	if len(diff) > 0 {
		t.Error("got diffs:", diff)
	}
}

func TestStruct(t *testing.T) {
	type s1 struct {
		id     int
		Name   string
		Number int
	}
	sa := s1{
		id:     1,
		Name:   "foo",
		Number: 2,
	}
	sb := sa
	diff, _ := deep.CompareS(sa, sb)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	sb.Name = "bar"
	diff, _ = deep.CompareS(sa, sb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "Name: foo != bar" {
		t.Error("wrong diff:", diff[0])
	}

	sb.Number = 22
	diff, _ = deep.CompareS(sa, sb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 2 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "Name: foo != bar" {
		t.Error("wrong diff:", diff[0])
	}
	if diff[1] != "Number: 2 != 22" {
		t.Error("wrong diff:", diff[1])
	}

	sb.id = 11
	diff, _ = deep.CompareS(sa, sb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 2 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "Name: foo != bar" {
		t.Error("wrong diff:", diff[0])
	}
	if diff[1] != "Number: 2 != 22" {
		t.Error("wrong diff:", diff[1])
	}
}

func TestNestedStruct(t *testing.T) {
	type s2 struct {
		Nickname string
	}
	type s1 struct {
		Name  string
		Alias s2
	}
	sa := s1{
		Name:  "Robert",
		Alias: s2{Nickname: "Bob"},
	}
	sb := sa
	diff, _ := deep.CompareS(sa, sb)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	sb.Alias.Nickname = "Bobby"
	diff, _ = deep.CompareS(sa, sb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "Alias.Nickname: Bob != Bobby" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestMap(t *testing.T) {
	ma := map[string]int{
		"foo": 1,
		"bar": 2,
	}
	mb := map[string]int{
		"foo": 1,
		"bar": 2,
	}
	diff, _ := deep.CompareS(ma, mb)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, _ = deep.CompareS(ma, ma)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	mb["foo"] = 111
	diff, _ = deep.CompareS(ma, mb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "foo: 1 != 111" {
		t.Error("wrong diff:", diff[0])
	}

	delete(mb, "foo")
	diff, _ = deep.CompareS(ma, mb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "foo: 1 != [empty value]" {
		t.Error("wrong diff:", diff[0])
	}

	diff, _ = deep.CompareS(mb, ma)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "foo: [empty value] != 1" {
		t.Error("wrong diff:", diff[0])
	}

	var mc map[string]int
	diff, _ = deep.CompareS(ma, mc)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	// handle hash order randomness
	if diff[0] != "map[foo:1 bar:2] != [empty value]" && diff[0] != "map[bar:2 foo:1] != [empty value]" {
		t.Error("wrong diff:", diff[0])
	}

	diff, _ = deep.CompareS(mc, ma)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "[empty value] != map[foo:1 bar:2]" && diff[0] != "[empty value] != map[bar:2 foo:1]" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestArray(t *testing.T) {
	a := [3]int{1, 2, 3}
	b := [3]int{1, 2, 3}

	diff, _ := deep.CompareS(a, b)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, _ = deep.CompareS(a, a)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	b[2] = 333
	diff, _ = deep.CompareS(a, b)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "#2: 3 != 333" {
		t.Error("wrong diff:", diff[0])
	}

	c := [3]int{1, 2, 2}
	diff, _ = deep.CompareS(a, c)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "#2: 3 != 2" {
		t.Error("wrong diff:", diff[0])
	}

	var d [2]int
	diff, _ = deep.CompareS(a, d)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "[3]int != [2]int" {
		t.Error("wrong diff:", diff[0])
	}

	e := [12]int{}
	f := [12]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	diff, _ = deep.CompareS(e, f)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != deep.DefaultOptions.MaxDiff {
		t.Error("not enough diffs:", diff)
	}
	for i := 0; i < deep.DefaultOptions.MaxDiff; i++ {
		if diff[i] != fmt.Sprintf("#%d: 0 != %d", i+1, i+1) {
			t.Error("wrong diff:", diff[i])
		}
	}
}

func TestSlice(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{1, 2, 3}

	diff, _ := deep.CompareS(a, b)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff, _ = deep.CompareS(a, a)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	b[2] = 333
	diff, _ = deep.CompareS(a, b)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "#2: 3 != 333" {
		t.Error("wrong diff:", diff[0])
	}

	b = b[0:2]
	diff, _ = deep.CompareS(a, b)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "#2: 3 != [empty value]" {
		t.Error("wrong diff:", diff[0])
	}

	diff, _ = deep.CompareS(b, a)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "#2: [empty value] != 3" {
		t.Error("wrong diff:", diff[0])
	}

	var c []int
	diff, _ = deep.CompareS(a, c)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "[1 2 3] != [empty value]" {
		t.Error("wrong diff:", diff[0])
	}

	diff, _ = deep.CompareS(c, a)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "[empty value] != [1 2 3]" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestPointer(t *testing.T) {
	type T struct {
		i int
	}
	a := &T{i: 1}
	b := &T{i: 1}
	diff, _ := deep.CompareS(a, b)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	a = nil
	diff, _ = deep.CompareS(a, b)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "<nil pointer> != deep_test.T" {
		t.Error("wrong diff:", diff[0])
	}

	a = b
	b = nil
	diff, _ = deep.CompareS(a, b)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "deep_test.T != <nil pointer>" {
		t.Error("wrong diff:", diff[0])
	}

	a = nil
	b = nil
	diff, _ = deep.CompareS(a, b)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}
}

func TestTime(t *testing.T) {
	// In an interable kind (i.e. a struct)
	type sTime struct {
		T time.Time
	}
	now := time.Now()
	got := sTime{T: now}
	expect := sTime{T: now.Add(1 * time.Second)}
	diff, _ := deep.CompareS(got, expect)
	if len(diff) != 1 {
		t.Error("expected 1 diff:", diff)
	}

	// Directly
	a := now
	b := now
	diff, _ = deep.CompareS(a, b)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	// https://github.com/go-test/deep/issues/15
	type Time15 struct {
		time.Time
	}
	a15 := Time15{now}
	b15 := Time15{now}
	diff, _ = deep.CompareS(a15, b15)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	later := now.Add(1 * time.Second)
	b15 = Time15{later}
	diff, _ = deep.CompareS(a15, b15)
	if len(diff) != 1 {
		t.Errorf("got %d diffs, expected 1: %s", len(diff), diff)
	}

	// No diff in deep.CompareS should not affect diff of other fields (Foo)
	type Time17 struct {
		time.Time
		Foo int
	}
	a17 := Time17{Time: now, Foo: 1}
	b17 := Time17{Time: now, Foo: 2}
	diff, _ = deep.CompareS(a17, b17)
	if len(diff) != 1 {
		t.Errorf("got %d diffs, expected 1: %s", len(diff), diff)
	}
}

func TestTimeUnexported(t *testing.T) {
	// https://github.com/go-test/deep/issues/18
	// Can't call Call() on exported Value func
	defaultCompareUnexportedFields := deep.DefaultOptions.CompareUnexportedFields
	deep.DefaultOptions.CompareUnexportedFields = true
	defer func() { deep.DefaultOptions.CompareUnexportedFields = defaultCompareUnexportedFields }()

	now := time.Now()
	type hiddenTime struct {
		t time.Time
	}
	htA := &hiddenTime{t: now}
	htB := &hiddenTime{t: now}
	diff, _ := deep.CompareS(htA, htB)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	// This doesn't call time.Time.deep.CompareS(), it compares the unexported fields
	// in time.Time, causing a diff like:
	// [t.wall: 13740788835924462040 != 13740788836998203864 t.ext: 1447549 != 1001447549]
	later := now.Add(1 * time.Second)
	htC := &hiddenTime{t: later}
	diff, _ = deep.CompareS(htA, htC)

	expected := 1
	if _, ok := reflect.TypeOf(htA.t).FieldByName("ext"); ok {
		expected = 2
	}
	if len(diff) != expected {
		t.Errorf("got %d diffs, expected %d: %s", len(diff), expected, diff)
	}
}

func TestInterface(t *testing.T) {
	a := map[string]interface{}{
		"foo": map[string]string{
			"bar": "a",
		},
	}
	b := map[string]interface{}{
		"foo": map[string]string{
			"bar": "b",
		},
	}
	diff, _ := deep.CompareS(a, b)
	if len(diff) == 0 {
		t.Fatalf("expected 1 diff, got zero")
	}
	if len(diff) != 1 {
		t.Errorf("expected 1 diff, got %d: %s", len(diff), diff)
	}
}

func TestInterface2(t *testing.T) {
	defer func() {
		if val := recover(); val != nil {
			t.Fatalf("panic: %v", val)
		}
	}()

	a := map[string]interface{}{
		"bar": 1,
	}
	b := map[string]interface{}{
		"bar": 1.23,
	}
	diff, _ := deep.CompareS(a, b)
	if len(diff) == 0 {
		t.Fatalf("expected 1 diff, got zero")
	}
	if len(diff) != 1 {
		t.Errorf("expected 1 diff, got %d: %s", len(diff), diff)
	}
}

func TestInterface3(t *testing.T) {
	type Value struct{ int }
	a := map[string]interface{}{
		"foo": &Value{},
	}
	b := map[string]interface{}{
		"foo": 1.23,
	}
	diff, _ := deep.CompareS(a, b)
	if len(diff) == 0 {
		t.Fatalf("expected 1 diff, got zero")
	}

	if len(diff) != 1 {
		t.Errorf("expected 1 diff, got: %s", diff)
	}
}

func TestError(t *testing.T) {
	a := errors.New("it broke")
	b := errors.New("it broke")

	diff, _ := deep.CompareS(a, b)
	if len(diff) != 0 {
		t.Fatalf("expected zero diffs, got %d: %s", len(diff), diff)
	}

	b = errors.New("it fell apart")
	diff, _ = deep.CompareS(a, b)
	if len(diff) != 1 {
		t.Fatalf("expected 1 diff, got %d: %s", len(diff), diff)
	}
	if diff[0] != "it broke != it fell apart" {
		t.Errorf("got '%s', expected 'it broke != it fell apart'", diff[0])
	}

	// Both errors set
	type tWithError struct {
		Error error
	}
	t1 := tWithError{
		Error: a,
	}
	t2 := tWithError{
		Error: b,
	}
	diff, _ = deep.CompareS(t1, t2)
	if len(diff) != 1 {
		t.Fatalf("expected 1 diff, got %d: %s", len(diff), diff)
	}
	if diff[0] != "Error: it broke != it fell apart" {
		t.Errorf("got '%s', expected 'Error: it broke != it fell apart'", diff[0])
	}

	// Both errors nil
	t1 = tWithError{
		Error: nil,
	}
	t2 = tWithError{
		Error: nil,
	}
	diff, _ = deep.CompareS(t1, t2)
	if len(diff) != 0 {
		t.Log(diff)
		t.Fatalf("expected 0 diff, got %d: %s", len(diff), diff)
	}

	// One error is nil
	t1 = tWithError{
		Error: errors.New("foo"),
	}
	t2 = tWithError{
		Error: nil,
	}
	diff, _ = deep.CompareS(t1, t2)
	if len(diff) != 1 {
		t.Log(diff)
		t.Fatalf("expected 1 diff, got %d: %s", len(diff), diff)
	}
	if diff[0] != "Error: *errors.errorString != <nil pointer>" {
		t.Errorf("got '%s', expected 'Error: *errors.errorString != <nil pointer>'", diff[0])
	}
}

func TestNil(t *testing.T) {
	type student struct {
		name string
		age  int
	}

	mark := student{"mark", 10}
	var someNilThing interface{} = nil
	diff, _ := deep.CompareS(someNilThing, mark)
	if diff == nil {
		t.Error("Nil value to comparison should not be equal")
	}
	diff, _ = deep.CompareS(mark, someNilThing)
	if diff == nil {
		t.Error("Nil value to comparison should not be equal")
	}
	diff, _ = deep.CompareS(someNilThing, someNilThing)
	if diff != nil {
		t.Error("Nil value to comparison should not be equal")
	}
}
