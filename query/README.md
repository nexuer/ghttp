# query

English | [简体中文](README_zh-CN.md)

`query` encodes Go data into `url.Values`. It supports structs, maps, query strings, and key/value sequences.

## Quick Start

```go
type Options struct {
    Keyword string   `query:"q"`
    Page    int      `query:"page"`
    Tags    []string `query:"tag"`
}

values, err := query.Values(Options{
    Keyword: "go",
    Page:    2,
    Tags:    []string{"client", "http"},
})
if err != nil {
    return err
}

fmt.Println(values.Encode())
// page=2&q=go&tag=client&tag=http
```

## Supported Inputs

| Input | Behavior |
| --- | --- |
| `struct` | Encodes exported fields with `query` or `url` tags. |
| `map[K]V` | Converts keys to strings and recursively encodes values. |
| `string`, `[]byte` | Parses a query string with optional leading `?` characters. |
| `url.Values` | Returns the original value without copying it. |
| `[]T`, `[N]T` | Encodes a top-level input as alternating key/value pairs. |
| `nil`, nil top-level pointer | Returns an empty `url.Values`. |

Non-nil pointers are dereferenced recursively.

A top-level slice or array requires an even number of elements:

```go
query.Values([]any{"name", "alice", "page", 2})
// name=alice&page=2
```

An odd number of elements returns an error. Use a struct or map for complex nested data.

## Struct Tags

A non-empty `query` tag takes precedence over `url`. The field name is used when the tag has no name. Use `query:"-"` or `url:"-"` to omit a field.

```go
type Request struct {
    Keyword string `query:"q"`
    Page    int    `url:"page"`
    Token   string `query:"-"`
}
```

Common options:

| Option | Behavior |
| --- | --- |
| `omitempty` | Omits an empty field. |
| `inline` | Removes the current level of a nested map or struct. Use `query:",inline"`. |
| `int` | Encodes a bool as `1` or `0`. |
| `unix` | Encodes `time.Time` in Unix seconds. |
| `unixmilli` | Encodes `time.Time` in Unix milliseconds. |
| `unixnano` | Encodes `time.Time` in Unix nanoseconds. |

Use a separate `layout` tag for custom time formatting:

```go
CreatedAt time.Time `query:"created_at" layout:"2006-01-02"`
```

## Empty Values

Zero-valued fields are encoded by default and omitted only with `omitempty`.

```text
false -> false
0     -> 0
""    -> empty query value
```

`omitempty` checks the field itself; it does not filter collection elements:

```text
[]string{}   -> no parameter
[]string{""} -> value=
[2]string{}  -> value=&value=
```

A nil pointer or nil interface field is encoded as an empty query value. A zero `time.Time` is also encoded as an empty value and is omitted with `omitempty`.

## Maps and Nested Values

Map keys are converted to strings. Values may contain maps, structs, slices, or arrays. Maps preserve `false`, `0`, empty strings, and nil values.

Nested parameters use bracket notation by default:

```text
user[name]=alice&user[address][city]=Shanghai
```

`inline` removes the current field level:

```go
User UserOptions `query:",inline"`
```

## Slice and Array Fields

Slice and array fields are encoded as repeated parameters by default:

```go
IDs []int `query:"id"`
// id=1&id=2
```

Formatting options:

| Option | Example                                  |
| --- |------------------------------------------|
| `comma` | `tag=a,b`                                |
| `space` | `tag=a b`                                |
| `semicolon` | `tag=a;b`                                |
| `brackets` | `tag[]=a&tag[]=b`                        |
| `numbered` | `tag0=a&tag1=b`                          |
| `idx` | `tag[0]=a&tag[1]=b`                      |
| `del:value` | `query:"tag,del:\|"` produces `tag=a\|b` |

A custom delimiter can also use a separate tag:

```go
Tags []string `query:"tag" del:"|"`
```

When multiple join options are present, precedence is `comma`, `space`, `semicolon`, `brackets`, then `del`. `numbered` takes precedence over `idx`.

## Custom Encoding

A struct field may implement `Encoder`:

```go
type Encoder interface {
    EncodeValues(key string, values *url.Values) error
}
```

Concrete values stored in interface fields are also checked for `Encoder`. Errors returned by `EncodeValues` are returned by `Values`.

## ScopeJoiner

`SetScopeJoiner` changes nested parameter names for both structs and maps:

```go
query.SetScopeJoiner(func(scope, name string) string {
    return scope + "." + name
})
// user.name
```

`SetScopeJoiner` may be called concurrently with `Values`. Each `Values` call uses one joiner snapshot. Passing `nil` restores the default bracket notation.

A custom joiner may be invoked concurrently by multiple `Values` calls. A callback that accesses mutable state is responsible for its own synchronization.

## Errors

`Values` returns an error when:

- the top-level input type is unsupported;
- a top-level slice or array contains an odd number of elements;
- a string or `[]byte` is not a valid query string;
- a custom `Encoder` returns an error.
