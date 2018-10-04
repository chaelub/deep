// Package deep provides function deep.Equal which is like reflect.DeepEqual but
// returns a list of differences. This is helpful when comparing complex types
// like structures and maps.
package deep

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
)

var (
	// ErrMaxRecursion is logged when MaxDepth is reached.
	ErrMaxRecursion = errors.New("recursed to MaxDepth")

	// ErrTypeMismatch is logged when Equal passed two different types of values.
	ErrTypeMismatch = errors.New("variables are different reflect.Type")

	// ErrNotHandled is logged when a primitive Go kind is not handled.
	ErrNotHandled = errors.New("cannot compare the reflect.Kind")

	DefaultOptions = Options{
		FloatPrecision:          10,
		MaxDiff:                 10,
		MaxDepth:                10,
		LogErrors:               false,
		CompareUnexportedFields: false,
	}
)

type Options struct {
	// FloatPrecision is the number of decimal places to round float values
	// to when comparing.
	FloatPrecision int
	// MaxDiff specifies the maximum number of differences to return.
	MaxDiff int
	// MaxDepth specifies the maximum levels of a struct to recurse into.
	MaxDepth int
	// LogErrors causes errors to be logged to STDERR when true.
	LogErrors bool
	// CompareUnexportedFields causes unexported struct fields, like s in
	// T{s int}, to be comparsed when true.
	CompareUnexportedFields bool

	asMap bool
}

type DiffResult struct {
	OldValue interface{}
	NewValue interface{}
}

type cmp struct {
	diff        []string
	diffM       map[string]DiffResult
	buff        []string
	floatFormat string
	opts        Options
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// Equal compares variables a and b, recursing into their structure up to
// MaxDepth levels deep, and returns a list of differences, or nil if there are
// none. Some differences may not be found if an error is also returned.
//
// If a type has an Equal method, like time.Equal, it is called to check for
// equality.

func CompareM(a, b interface{}, opts ...Options) (map[string]DiffResult, bool) {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	} else {
		o = DefaultOptions
	}
	o.asMap = true
	if c, hasDiff := compare(a, b, o); hasDiff {
		return c.diffM, hasDiff
	}
	return nil, false
}

func CompareS(a, b interface{}, opts ...Options) ([]string, bool) {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	} else {
		o = DefaultOptions
	}
	if c, hasDiff := compare(a, b, o); hasDiff {
		return c.diff, hasDiff
	}
	return nil, false
}

func compare(a, b interface{}, opts Options) (c *cmp, hasDiff bool) {
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)
	c = &cmp{
		diff:        []string{},
		diffM:       make(map[string]DiffResult),
		buff:        []string{},
		opts:        opts,
		floatFormat: fmt.Sprintf("%%.%df", opts.FloatPrecision),
	}

	if a == nil && b == nil {
		return
	} else if a == nil && b != nil {
		c.saveDiff(b, "<nil pointer>")
	} else if a != nil && b == nil {
		c.saveDiff(a, "<nil pointer>")
	}
	if len(c.diff) > 0 {
		return c, true
	}

	c.equals(aVal, bVal, 0)
	if len(c.diff) > 0 || len(c.diffM) > 0 {
		return c, true
	}
	return
}

func (c *cmp) equals(a, b reflect.Value, level int) {
	if level > c.opts.MaxDepth {
		c.logError(ErrMaxRecursion)
		return
	}

	// Check if one value is nil, e.g. T{x: *X} and T.x is nil
	if !a.IsValid() || !b.IsValid() {
		if a.IsValid() && !b.IsValid() {
			c.saveDiff(a.Type(), "<nil pointer>")
		} else if !a.IsValid() && b.IsValid() {
			c.saveDiff("<nil pointer>", b.Type())
		}
		return
	}

	// If differenet types, they can't be equal
	aType := a.Type()
	bType := b.Type()
	if aType != bType {
		c.saveDiff(aType, bType)
		c.logError(ErrTypeMismatch)
		return
	}

	// Primitive https://golang.org/pkg/reflect/#Kind
	aKind := a.Kind()
	bKind := b.Kind()

	// If both types implement the error interface, compare the error strings.
	// This must be done before dereferencing because the interface is on a
	// pointer receiver.
	if aType.Implements(errorType) && bType.Implements(errorType) {
		if a.Elem().IsValid() && b.Elem().IsValid() { // both err != nil
			aString := a.MethodByName("Error").Call(nil)[0].String()
			bString := b.MethodByName("Error").Call(nil)[0].String()
			if aString != bString {
				c.saveDiff(aString, bString)
			}
			return
		}
	}

	// Dereference pointers and interface{}
	if aElem, bElem := (aKind == reflect.Ptr || aKind == reflect.Interface),
		(bKind == reflect.Ptr || bKind == reflect.Interface); aElem || bElem {

		if aElem {
			a = a.Elem()
		}

		if bElem {
			b = b.Elem()
		}

		c.equals(a, b, level+1)
		return
	}

	switch aKind {

	/////////////////////////////////////////////////////////////////////
	// Iterable kinds
	/////////////////////////////////////////////////////////////////////

	case reflect.Struct:
		/*
			The variables are structs like:
				type T struct {
					FirstName string
					LastName  string
				}
			Type = <pkg>.T, Kind = reflect.Struct

			Iterate through the fields (FirstName, LastName), recurse into their values.
		*/

		// Types with an Equal() method, like time.Time, only if struct field
		// is exported (CanInterface)
		if eqFunc := a.MethodByName("Equal"); eqFunc.IsValid() && eqFunc.CanInterface() {
			// Handle https://github.com/go-test/deep/issues/15:
			// Don't call T.Equal if the method is from an embedded struct, like:
			//   type Foo struct { time.Time }
			// First, we'll encounter Equal(Ttime, time.Time) but if we pass b
			// as the 2nd arg we'll panic: "Call using pkg.Foo as type time.Time"
			// As far as I can tell, there's no way to see that the method is from
			// time.Time not Foo. So we check the type of the 1st (0) arg and skip
			// unless it's b type. Later, we'll encounter the time.Time anonymous/
			// embedded field and then we'll have Equal(time.Time, time.Time).
			funcType := eqFunc.Type()
			if funcType.NumIn() == 1 && funcType.In(0) == bType {
				retVals := eqFunc.Call([]reflect.Value{b})
				if !retVals[0].Bool() {
					c.saveDiff(a, b)
				}
				return
			}
		}

		for i := 0; i < a.NumField(); i++ {
			if aType.Field(i).PkgPath != "" && !c.opts.CompareUnexportedFields {
				continue // skip unexported field, e.g. s in type T struct {s string}
			}

			tagOpts := getTagOpts(aType.Field(i).Tag.Get("compare"))
			if tagOpts.skip {
				continue
			}

			// push field name to buff
			if tagOpts.exists {
				c.push(tagOpts.name)
			} else {
				c.push(aType.Field(i).Name)
			}

			// Get the Value for each field, e.g. FirstName has Type = string,
			// Kind = reflect.String.
			af := a.Field(i)
			bf := b.Field(i)

			// Recurse to compare the field values
			c.equals(af, bf, level+1)

			c.pop() // pop field name from buff

			if len(c.diff) >= c.opts.MaxDiff {
				break
			}
		}
	case reflect.Map:
		/*
			The variables are maps like:
				map[string]int{
					"foo": 1,
					"bar": 2,
				}
			Type = map[string]int, Kind = reflect.Map

			Or:
				type T map[string]int{}
			Type = <pkg>.T, Kind = reflect.Map

			Iterate through the map keys (foo, bar), recurse into their values.
		*/

		if a.IsNil() || b.IsNil() {
			if a.IsNil() && !b.IsNil() {
				c.saveDiff("[empty value]", b.Interface())
			} else if !a.IsNil() && b.IsNil() {
				c.saveDiff(a.Interface(), "[empty value]")
			}
			return
		}

		if a.Pointer() == b.Pointer() {
			return
		}

		for _, key := range a.MapKeys() {
			c.push(fmt.Sprintf("%s", key))

			aVal := a.MapIndex(key)
			bVal := b.MapIndex(key)
			if bVal.IsValid() {
				c.equals(aVal, bVal, level+1)
			} else {
				c.saveDiff(aVal.Interface(), "[empty value]")
			}

			c.pop()

			if len(c.diff) >= c.opts.MaxDiff {
				return
			}
		}

		for _, key := range b.MapKeys() {
			if aVal := a.MapIndex(key); aVal.IsValid() {
				continue
			}

			c.push(key.String())
			c.saveDiff("[empty value]", b.MapIndex(key).Interface())
			c.pop()
			if len(c.diff) >= c.opts.MaxDiff {
				return
			}
		}
	case reflect.Array:
		n := a.Len()
		for i := 0; i < n; i++ {
			c.push(fmt.Sprintf("#%d", i))
			c.equals(a.Index(i), b.Index(i), level+1)
			c.pop()
			if len(c.diff) >= c.opts.MaxDiff {
				break
			}
		}
	case reflect.Slice:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() && !b.IsNil() {
				c.saveDiff("[empty value]", b)
			} else if !a.IsNil() && b.IsNil() {
				c.saveDiff(a, "[empty value]")
			}
			return
		}

		if a.Pointer() == b.Pointer() {
			return
		}

		aLen := a.Len()
		bLen := b.Len()
		n := aLen
		if bLen > aLen {
			n = bLen
		}
		for i := 0; i < n; i++ {
			c.push(fmt.Sprintf("#%d", i))
			if i < aLen && i < bLen {
				c.equals(a.Index(i), b.Index(i), level+1)
			} else if i < aLen {
				c.saveDiff(a.Index(i), "[empty value]")
			} else {
				c.saveDiff("[empty value]", b.Index(i))
			}
			c.pop()
			if len(c.diff) >= c.opts.MaxDiff {
				break
			}
		}

	/////////////////////////////////////////////////////////////////////
	// Primitive kinds
	/////////////////////////////////////////////////////////////////////

	case reflect.Float32, reflect.Float64:
		// Avoid 0.04147685731961082 != 0.041476857319611
		// 6 decimal places is close enough
		aval := fmt.Sprintf(c.floatFormat, a.Float())
		bval := fmt.Sprintf(c.floatFormat, b.Float())
		if aval != bval {
			c.saveDiff(a.Float(), b.Float())
		}
	case reflect.Bool:
		if a.Bool() != b.Bool() {
			c.saveDiff(a.Bool(), b.Bool())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if a.Int() != b.Int() {
			c.saveDiff(a.Int(), b.Int())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if a.Uint() != b.Uint() {
			c.saveDiff(a.Uint(), b.Uint())
		}
	case reflect.String:
		if a.String() != b.String() {
			c.saveDiff(a.String(), b.String())
		}

	default:
		c.logError(ErrNotHandled)
	}
}

func (c *cmp) push(name string) {
	c.buff = append(c.buff, name)
}

func (c *cmp) pop() {
	if len(c.buff) > 0 {
		c.buff = c.buff[0 : len(c.buff)-1]
	}
}

func (c *cmp) saveDiff(aval, bval interface{}) {
	if len(c.buff) > 0 {
		varName := strings.Join(c.buff, ".")
		if c.opts.asMap {
			c.diffM[varName] = DiffResult{
				OldValue: aval,
				NewValue: bval,
			}
			return
		}
		c.diff = append(c.diff, fmt.Sprintf("%s: %v != %v", varName, aval, bval))
	} else {
		if c.opts.asMap {
			c.diffM["result"] = DiffResult{
				OldValue: aval,
				NewValue: bval,
			}
		}
		c.diff = append(c.diff, fmt.Sprintf("%v != %v", aval, bval))
	}
}

func (c *cmp) logError(err error) {
	if c.opts.LogErrors {
		log.Println(err)
	}
}

type tagOptions struct {
	exists bool
	name   string
	skip   bool
}

func getTagOpts(tagV string) tagOptions {
	opts := tagOptions{}
	if len(tagV) > 0 {
		opts.exists = true
	}
	if od := strings.Split(tagV, ","); len(od) > 1 {
		opts.name = od[0]
		opts.skip = true
	} else {
		opts.name = tagV
	}
	return opts
}
