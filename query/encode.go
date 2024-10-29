package query

import (
	"bytes"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var defaultScopeJoiner ScopeJoiner = func(scope, name string) string {
	return scope + "[" + name + "]"
}

type ScopeJoiner func(scope, name string) string

func SetScopeJoiner(sj ScopeJoiner) {
	defaultScopeJoiner = sj
}

var tags = [2]string{"query", "url"}

var encoderType = reflect.TypeOf(new(Encoder)).Elem()

// Encoder is an interface implemented by any type that wishes to encode
// itself into URL values in a non-standard way.
type Encoder interface {
	EncodeValues(key string, v *url.Values) error
}

var timeType = reflect.TypeOf(time.Time{})

// Values returns the url.Values encoding of v.
//
// Values expects to be passed a struct, string, map, array, or slice,
// and traverses it recursively using the following encoding
// rules for structs.See examples in encode_test.go:
// - string: 		 TestValues_string
// - map: 			 TestValues_map
// - array or slice: TestValues_array_or_slice
// - struct:
//
// Struct Tag: `query:""` or `url:""`,  with `query` taking precedence over `url`.
// For example, if both tags  are present, the `query` tag will be used for encoding.
// Each exported struct field is encoded as a URL parameter unless
//
//   - the field's tag is "-", or
//   - the field is empty and its tag specifies the "omitempty" option
//
// The empty values are false, 0, any nil pointer or interface value, any array
// slice, map, or string of length zero, and any type (such as time.Time) that
// returns true for IsZero().
//
// The URL parameter name defaults to the struct field name but can be
// specified in the struct field's tag value.  The "query" key in the struct
// field's tag value is the key name, followed by an optional comma and
// options.  For example:
//
//	// Field is ignored by this package.
//	Field int `query:"-"`
//
//	// Field appears as URL parameter "myName".
//	Field int `query:"myName"`
//
//	// Field appears as URL parameter "myName" and the field is omitted if
//	// its value is empty
//	Field int `query:"myName,omitempty"`
//
//	// Field appears as URL parameter "Field" (the default), but the field
//	// is skipped if empty.  Note the leading comma.
//	Field int `query:",omitempty"`
//
// For encoding individual field values, the following type-dependent rules
// apply:
//
// Boolean values default to encoding as the strings "true" or "false".
// Including the "int" option signals that the field should be encoded as the
// strings "1" or "0".
//
// time.Time values default to encoding as RFC3339 timestamps.  Including the
// "unix" option signals that the field should be encoded as a Unix time (see
// time.Unix()).  The "unixmilli" and "unixnano" options will encode the number
// of milliseconds and nanoseconds, respectively, since January 1, 1970 (see
// time.UnixNano()).  Including the "layout" struct tag (separate from the
// "query" tag) will use the value of the "layout" tag as a layout passed to
// time.Format.  For example:
//
//	// Encode a time.Time as YYYY-MM-DD HH:ii:ss
//	Field time.Time `layout:"2006-01-02 15:04:05"`
//
// Slice and Array values default to encoding as multiple URL values of the
// same name.  Including the "comma" option signals that the field should be
// encoded as a single comma-delimited value.  Including the "space" option
// similarly encodes the value as a single space-delimited string. Including
// the "semicolon" option will encode the value as a semicolon-delimited string.
// Including the "brackets" option signals that the multiple URL values should
// have "[]" appended to the value name. "numbered" will append a number to
// the end of each incidence of the value name, example:
// name0=value0&name1=value1, etc.  Including the "del" struct tag (separate
// from the "query" tag) will use the value of the "del" tag as the delimiter.
// For example:
//
//	// Encode a slice of bools as ints ("1" for true, "0" for false),
//	// separated by exclamation points "!".
//	Field []bool `query:",int,del:!"`
//
// Anonymous struct fields are usually encoded as if their inner exported
// fields were fields in the outer struct, subject to the standard Go
// visibility rules.  An anonymous struct field with a name given in its URL
// tag is treated as having that name, rather than being anonymous.
//
// Non-nil pointer values are encoded as the value pointed to.
//
// Nested structs have their fields processed recursively and are encoded
// including parent fields in value names for scoping. For example,
//
//	"user[name]=acme&user[addr][postcode]=1234&user[addr][city]=SFO"
//
// If "User" tag with query:",inline", the encoding would look like:
//
//	"name=acme&addr[postcode]=1234&addr[city]=SFO"
//
// All other values are encoded using their default string representation.
//
// Multiple fields that encode to the same URL parameter name will be included
// as multiple URL values of the same name.
func Values(v interface{}) (url.Values, error) {
	values := make(url.Values)

	if v == nil {
		return values, nil
	}

	val := reflect.ValueOf(v)
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return values, nil
		}
		val = val.Elem()
	}

	switch str := val.Interface().(type) {
	case string:
		return parseQueryString(str)
	case []byte:
		return parseQueryString(string(str))
	case url.Values:
		return str, nil
	}

	err := reflectValue(values, val)
	return values, err
}

func parseQueryString(queryString string) (url.Values, error) {
	return url.ParseQuery(strings.TrimLeft(queryString, "?"))
}

func reflectValue(values url.Values, val reflect.Value) error {
	switch val.Kind() {
	case reflect.Map:
		return reflectMap(values, val, "", 0, nil)
	case reflect.Slice, reflect.Array:
		return reflectSlice(values, val, "", 0, nil)
	case reflect.Struct:
		return reflectStruct(values, val, "", 0)
	default:
		return fmt.Errorf("query: Values() unsupported kind input. Got %v", val.Kind())
	}
}

// reflectValue populates the values parameter from the struct fields in val.
// Embedded structs are followed recursively (using the rules defined in the
// Values function documentation) breadth-first.
func reflectStruct(values url.Values, val reflect.Value, scope string, count int) error {
	var embedded []reflect.Value

	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		sf := typ.Field(i)
		if sf.PkgPath != "" && !sf.Anonymous { // unexported
			continue
		}
		// reset tag
		tag := ""
		for _, tn := range tags {
			tag = sf.Tag.Get(tn)
			if tag != "" {
				break
			}
		}

		if tag == "-" {
			continue
		}

		sv := val.Field(i)
		fieldName, opts := parseTag(tag, sf)
		name := fieldName
		if name == "" {
			if sf.Anonymous {
				v := reflect.Indirect(sv)
				if v.IsValid() && v.Kind() == reflect.Struct {
					// save embedded struct for later processing
					embedded = append(embedded, v)
					continue
				}
			}

			name = sf.Name
		}

		if scope != "" {
			name = scope + "[" + name + "]"
		}

		if opts.contains("omitempty") && isEmptyValue(sv) {
			continue
		}

		if sv.Type().Implements(encoderType) {
			// if sv is a nil pointer and the custom encoder is defined on a non-pointer
			// method receiver, set sv to the zero value of the underlying type
			if !reflect.Indirect(sv).IsValid() && sv.Type().Elem().Implements(encoderType) {
				sv = reflect.New(sv.Type().Elem())
			}

			m := sv.Interface().(Encoder)
			if err := m.EncodeValues(name, &values); err != nil {
				return err
			}
			continue
		}

		// recursively dereference pointers. break on nil pointers
		if sv.Kind() == reflect.Interface {
			sv = sv.Elem()
		}

		for sv.Kind() == reflect.Ptr {
			if sv.IsNil() {
				break
			}
			sv = sv.Elem()
		}

		// handle special types
		if sv.Type() == timeType {
			values.Add(name, valueString(sv, opts))
			continue
		}

		switch sv.Kind() {
		case reflect.Slice, reflect.Array:
			l := sv.Len()
			if l == 0 {
				// skip if slice or array is empty
				continue
			}

			var del string
			if opts.contains("comma") {
				del = ","
			} else if opts.contains("space") {
				del = " "
			} else if opts.contains("semicolon") {
				del = ";"
			} else if opts.contains("brackets") {
				name = name + "[]"
			} else {
				del = sf.Tag.Get("del")
			}

			if del != "" {
				s := new(bytes.Buffer)
				first := true
				for j := 0; j < l; j++ {
					if first {
						first = false
					} else {
						s.WriteString(del)
					}

					s.WriteString(valueString(sv.Index(j), opts))
				}
				values.Add(name, s.String())
			} else {
				for j := 0; j < l; j++ {
					k := name
					if opts.contains("numbered") {
						k = fmt.Sprintf("%s%d", name, j)
					} else if opts.contains("idx") {
						k = fmt.Sprintf("%s[%d]", name, j)
					}

					already, err := handleSliceValue(values, sv.Index(j), k, count, opts)
					if err != nil {
						return err
					}

					if !already {
						values.Add(k, valueString(sv.Index(j), opts))
					}
				}
			}
		case reflect.Map, reflect.Struct:
			nextScope := name
			if ok := opts.contains("inline"); fieldName == "" && ok {
				if count > 0 {
					nextScope = scope
				} else {
					nextScope = ""
				}
			}
			if sv.Kind() == reflect.Map {
				if err := reflectMap(values, sv, nextScope, count+1, opts); err != nil {
					return err
				}
			} else {
				if err := reflectStruct(values, sv, nextScope, count+1); err != nil {
					return err
				}
			}

		default:
			values.Add(name, valueString(sv, opts))
		}
	}

	for _, f := range embedded {
		if err := reflectStruct(values, f, scope, count); err != nil {
			return err
		}
	}

	return nil
}

type zeroable interface {
	IsZero() bool
}

// isEmptyValue checks if a value should be considered empty for the purposes
// of omitting fields with the "omitempty" option.
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Invalid:
		return true
	default:
		if z, ok := v.Interface().(zeroable); ok {
			return z.IsZero()
		}
	}

	return false
}

func handleSliceValue(values url.Values, sv reflect.Value, scope string, count int, opts *tagOptions) (bool, error) {
	if isEmptyValue(sv) {
		return true, nil
	}
	// recursively dereference pointers. break on nil pointers
	if sv.Kind() == reflect.Interface {
		sv = sv.Elem()
	}

	for sv.Kind() == reflect.Ptr {
		if sv.IsNil() {
			break
		}
		sv = sv.Elem()
	}

	switch sv.Kind() {
	case reflect.Map:
		if err := reflectMap(values, sv, scope, count+1, opts); err != nil {
			return false, err
		}
	case reflect.Slice, reflect.Array:
		if err := reflectSlice(values, sv, scope, count+1, opts); err != nil {
			return false, err
		}
	case reflect.Struct:
		if sv.Type() == timeType {
			return false, nil
		}
		if err := reflectStruct(values, sv, scope, count+1); err != nil {
			return false, err
		}
	default:
		return false, nil
	}

	return true, nil
}

func reflectSlice(values url.Values, val reflect.Value, scope string, count int, opts *tagOptions) error {
	l := val.Len()
	if l == 0 {
		return nil
	}
	for i := 0; i < l; i++ {
		sv := val.Index(i)

		already, err := handleSliceValue(values, sv, scope, count, opts)
		if err != nil {
			return err
		}

		if already {
			continue
		}

		endIndex := i + 1
		if endIndex > l {
			continue
		}
		if scope != "" {
			values.Add(scope, valueString(sv, nil))
			values.Add(scope, valueString(val.Index(endIndex), nil))
		} else {
			if endIndex > l-1 {
				continue
			}
			key := valueString(sv, nil)
			values.Add(key, valueString(val.Index(endIndex), nil))
		}
		i++
	}
	return nil
}

func reflectMap(values url.Values, val reflect.Value, scope string, count int, opts *tagOptions) error {
	iter := val.MapRange()
	for iter.Next() {
		sv := iter.Value()
		if isEmptyValue(sv) {
			continue
		}

		key := valueString(iter.Key(), nil)
		if scope != "" {
			key = defaultScopeJoiner(scope, key)
		}

		// recursively dereference pointers. break on nil pointers
		if sv.Kind() == reflect.Interface {
			sv = sv.Elem()
		}
		for sv.Kind() == reflect.Ptr {
			if sv.IsNil() {
				break
			}
			sv = sv.Elem()
		}

		if sv.Type() == timeType {
			values.Add(key, valueString(sv, opts))
			continue
		}

		switch sv.Kind() {
		case reflect.Map:
			if err := reflectMap(values, sv, key, count+1, opts); err != nil {
				return err
			}
		case reflect.Slice, reflect.Array:
			if err := reflectSlice(values, sv, key, count+1, opts); err != nil {
				return err
			}
		case reflect.Struct:
			if err := reflectStruct(values, sv, key, count+1); err != nil {
				return err
			}
		default:
			values.Add(key, valueString(sv, opts))
		}

	}
	return nil
}

// valueString returns the string representation of a value
func valueString(v reflect.Value, opts *tagOptions) string {
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	if v.Type() == timeType {
		t := v.Interface().(time.Time)
		if t.IsZero() {
			return ""
		}

		if opts != nil {
			// query:"create_time,unix"
			if opts.contains("unix") {
				return strconv.FormatInt(t.Unix(), 10)
			}
			// query:"create_time,unixmilli"
			if opts.contains("unixmilli") {
				return strconv.FormatInt(t.UnixNano()/1e6, 10)
			}
			// query:"create_time,unixnano"
			if opts.contains("unixnano") {
				return strconv.FormatInt(t.UnixNano(), 10)
			}
			// query:"create_time:" layout:"2006-01-02 15:04:05"
			if layout := opts.sf.Tag.Get("layout"); layout != "" {
				return t.Format(layout)
			}
		}
		return t.Format(time.RFC3339)
	}

	// query:"name,int"
	if opts != nil && v.Kind() == reflect.Bool && opts.contains("int") {
		if v.Bool() {
			return "1"
		}
		return "0"
	}

	// bytes to string
	if b, ok := v.Interface().([]byte); ok {
		return string(b)
	}

	return fmt.Sprint(v.Interface())
}

// tagOptions is the string following a comma in a struct field's "query" "url" tag, or
// the empty string. It does not include the leading comma.
type tagOptions struct {
	kvs map[string]string
	sf  reflect.StructField
}

// parseTag splits a struct field's url tag into its name and comma-separated
// options.
func parseTag(tag string, sf reflect.StructField) (string, *tagOptions) {
	s := strings.Split(tag, ",")
	opts := s[1:]
	tagOpts := &tagOptions{
		kvs: make(map[string]string),
		sf:  sf,
	}
	if len(opts) > 0 {
		for _, v := range opts {
			if v == "" {
				continue
			}
			keys := strings.Split(v, ":")
			if len(keys) == 0 {
				continue
			}
			tagOpts.kvs[keys[0]] = strings.Join(keys[1:], ":")
		}
	}
	return s[0], tagOpts
}

// contains checks whether the tagOptions contains the specified option.
func (o *tagOptions) contains(option string) bool {
	if o == nil {
		return false
	}
	_, ok := o.kvs[option]
	return ok
}

func (o *tagOptions) get(option string) string {
	if o == nil {
		return ""
	}
	return o.kvs[option]
}
