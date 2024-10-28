# query

`Query` is a library for encoding structs, strings, maps, arrays, or slices into URL query parameters.

## Supported Types

- `string`
- `map[any]any`
- `[]any` (slices)
- `[...]any` (arrays)
- `struct`

## Struct Tags
> Struct Tag: `query` or `url`, with `query` taking precedence over `url`. For example, if both tags are present, the `query` tag will be used for encoding.

Use struct tags for finer control, formatted as follows:

```text
`query:"yourName,inline,omitempty,comma,space,semicolon,brackets,int,unix" layout:"2006-01-02" del:","`
```
Or
```text
`url:"yourName,inline,omitempty,comma,space,semicolon,brackets,int,unix" layout:"2006-01-02" del:","`
```

### Tag Options

- `yourName`: Custom name (use - to ignore). If not set, the field name is used; if set to "-,", then "-" is the name.
- `omitempty`: Ignore this field if the value is empty.
- `inline`: Using inline makes nested structs level with parent structs.

Boolean
- `int`: For boolean types, true encodes to 1, and false encodes to 0.

Time
- `unix`: For time.Time types, returns the timestamp (in seconds).
- `unixmilli`: For time.Time types, returns the timestamp (in milliseconds).
- `unixnano`: For time.Time types, returns the timestamp (in nanoseconds).
- `layout`: Custom time format string.


For slices and arrays, you can use the following options for joining:

- `comma`: Joined with ",".
- `space`: Joined with " ".
- `semicolon`: Joined with ";".
- `brackets`: Array format, e.g., user[]=linda&user[]=liming.
- `del`: Custom delimiter, can be any value.

Nested Structs

Nested structs have their fields processed recursively and are encoded, including parent fields in value names for scoping. For example,
```text
"user[name]=acme&user[addr][postcode]=1234&user[addr][city]=SFO"
```
If the `User` tag includes `query:",inline"`, the encoding would look like:

```text
"name=acme&addr[postcode]=1234&addr[city]=SFO"
```

## Custom Encode
The Encoder interface allows types to implement custom encoding into URL values.
```go
type Encoder interface {
	EncodeValues(key string, v *url.Values) error
}
```
For example: 
```go
type customEncodedStrings []string

// EncodeValues using key name of the form "{key}.N" where N increments with
// each value.  A value of "err" will return an error.
func (m customEncodedStrings) EncodeValues(key string, v *url.Values) error {
	for i, arg := range m {
		if arg == "err" {
			return errors.New("encoding error")
		}
		v.Set(fmt.Sprintf("%s.%d", key, i), arg)
	}
	return nil
}
```
