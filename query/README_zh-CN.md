# query

[English](README.md) | 简体中文

`query` 将 Go 数据编码为 `url.Values`，支持结构体、map、查询字符串和键值序列。

## 快速开始

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

## 支持的输入

| 输入 | 行为 |
| --- | --- |
| `struct` | 编码导出字段，支持 `query` 和 `url` tag。 |
| `map[K]V` | key 转为字符串，value 递归编码。 |
| `string`、`[]byte` | 作为查询字符串解析，可带前导 `?`。 |
| `url.Values` | 直接返回原对象，不复制。 |
| `[]T`、`[N]T` | 顶层输入按交替的 key/value 对编码。 |
| `nil`、顶层 nil 指针 | 返回空的 `url.Values`。 |

非 nil 指针会被逐层解引用。

顶层 slice/array 要求包含偶数个元素：

```go
query.Values([]any{"name", "alice", "page", 2})
// name=alice&page=2
```

元素数量为奇数时返回错误。复杂嵌套数据建议使用 struct 或 map。

## 结构体标签

非空的 `query` tag 优先于 `url` tag。没有指定参数名时使用字段名，`query:"-"` 或 `url:"-"` 忽略字段。

```go
type Request struct {
    Keyword string `query:"q"`
    Page    int    `url:"page"`
    Token   string `query:"-"`
}
```

常用选项：

| 选项 | 作用 |
| --- | --- |
| `omitempty` | 字段为空时省略。 |
| `inline` | 去掉嵌套 map 或 struct 的当前字段层级，写作 `query:",inline"`。 |
| `int` | 将 bool 编码为 `1` 或 `0`。 |
| `unix` | 将 `time.Time` 编码为 Unix 秒。 |
| `unixmilli` | 将 `time.Time` 编码为 Unix 毫秒。 |
| `unixnano` | 将 `time.Time` 编码为 Unix 纳秒。 |

`layout` 使用独立 tag：

```go
CreatedAt time.Time `query:"created_at" layout:"2006-01-02"`
```

## 空值

字段零值默认会被编码，只有带 `omitempty` 时才省略。

```text
false -> false
0     -> 0
""    -> 空参数值
```

`omitempty` 判断字段本身，不过滤集合元素：

```text
[]string{}   -> 不产生参数
[]string{""} -> value=
[2]string{}  -> value=&value=
```

nil 指针或 nil interface 字段默认编码为空参数值。零值 `time.Time` 编码为空参数值；带 `omitempty` 时省略。

## map 和嵌套值

map 的 key 会转为字符串，value 可以嵌套 map、struct、slice 或 array。map 中的 `false`、`0`、空字符串和 nil 都会保留。

嵌套参数默认使用方括号：

```text
user[name]=alice&user[address][city]=Shanghai
```

`inline` 可以去掉当前字段层级：

```go
User UserOptions `query:",inline"`
```

## slice 和 array 字段

字段中的 slice/array 默认编码为同名多值：

```go
IDs []int `query:"id"`
// id=1&id=2
```

格式选项：

| 选项 | 示例                                 |
| --- |------------------------------------|
| `comma` | `tag=a,b`                          |
| `space` | `tag=a b`                          |
| `semicolon` | `tag=a;b`                          |
| `brackets` | `tag[]=a&tag[]=b`                  |
| `numbered` | `tag0=a&tag1=b`                    |
| `idx` | `tag[0]=a&tag[1]=b`                |
| `del:value` | `query:"tag,del:\|"` 得到 `tag=a\|b` |

自定义分隔符也可以使用独立 tag：

```go
Tags []string `query:"tag" del:"|"`
```

同时设置多个连接选项时，优先级为 `comma`、`space`、`semicolon`、`brackets`、`del`；`numbered` 优先于 `idx`。

## 自定义编码

结构体字段可以实现 `Encoder`：

```go
type Encoder interface {
    EncodeValues(key string, values *url.Values) error
}
```

interface 字段中的具体值也会检查是否实现了 `Encoder`。`EncodeValues` 返回的错误会由 `Values` 返回。

## ScopeJoiner

`SetScopeJoiner` 修改 struct 和 map 的嵌套参数名：

```go
query.SetScopeJoiner(func(scope, name string) string {
    return scope + "." + name
})
// user.name
```

`SetScopeJoiner` 可以与 `Values` 并发调用。每次 `Values` 使用同一个 joiner 快照。传入 `nil` 恢复默认方括号格式。

自定义 joiner 可能被多个 `Values` 并发调用；如果回调内部读写状态，需要自行保证并发安全。

## 错误

以下情况会返回错误：

- 顶层输入类型不支持。
- 顶层 slice/array 的元素数量为奇数。
- string 或 `[]byte` 不是合法查询字符串。
- 自定义 `Encoder` 返回错误。
