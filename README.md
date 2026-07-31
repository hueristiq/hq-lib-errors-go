# hq-lib-errors-go

![made with go](https://img.shields.io/badge/made%20with-Go-1E90FF.svg) [![go reference](https://pkg.go.dev/badge/github.com/hueristiq/hq-lib-errors-go.svg)](https://pkg.go.dev/github.com/hueristiq/hq-lib-errors-go) [![license](https://img.shields.io/badge/license-MIT-gray.svg?color=1E90FF)](https://github.com/hueristiq/hq-lib-errors-go/blob/main/LICENSE) ![maintenance](https://img.shields.io/badge/maintained%3F-yes-1E90FF.svg) [![open issues](https://img.shields.io/github/issues-raw/hueristiq/hq-lib-errors-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-errors-go/issues?q=is:issue+is:open) [![closed issues](https://img.shields.io/github/issues-closed-raw/hueristiq/hq-lib-errors-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-errors-go/issues?q=is:issue+is:closed) [![contribution](https://img.shields.io/badge/contributions-welcome-1E90FF.svg)](https://github.com/hueristiq/hq-lib-errors-go/blob/main/CONTRIBUTING.md)

`hq-lib-errors-go` is a [Go](https://golang.org/) package for rich, structured error handling.

## Resources

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
	- [Creating Errors](#creating-errors)
	- [Wrapping Errors](#wrapping-errors)
	- [Joining Multiple Errors](#joining-multiple-errors)
	- [Types & Structured Fields](#types--structured-fields)
	- [Unwrapping, `Is`, `As`, and `Cause`](#unwrapping-is-as-and-cause)
	- [Formatting Errors](#formatting-errors)
		- [... to String](#-to-string)
		- [... to JSON](#-to-json)
- [Contributing](#contributing)
- [Licensing](#licensing)

## Features

- **Stack Traces:** Every error captures where it was created.
- **Classification:** Tag errors with a `Type` for programmatic handling.
- **Structured Fields:** Attach arbitrary key-value metadata for richer debugging and structured logs.
- **Flexible Formatting:** Render an error chain as a human-readable string or a JSON-ready map.
- **Multi-Error Aggregation:** `Join` combines several errors into one that unwraps to all of them and records where the join happened.

## Installation

To install `hq-lib-errors-go`, run the following command in your Go project:

```bash
go get -v -u github.com/hueristiq/hq-lib-errors-go
```

## Usage

The examples below import the package under the `hqgoerrors` alias.

```go
import hqgoerrors "github.com/hueristiq/hq-lib-errors-go"
```

### Creating Errors

Use `New` to create a root error. A stack trace is captured at the point of invocation.

```go
package main

import (
	"fmt"

	hqgoerrors "github.com/hueristiq/hq-lib-errors-go"
)

func main() {
	err := hqgoerrors.New("failed to initialize database")
	if err != nil {
		fmt.Println(err.Error())
	}
}
```

### Wrapping Errors

Use `Wrap` to add context to an existing error. The new error captures its own wrap-site frame; the wrapped error is referenced as-is and is **never mutated**, so wrapping a shared or sentinel error is safe.

```go
package main

import (
	"fmt"

	hqgoerrors "github.com/hueristiq/hq-lib-errors-go"
)

func loadConfig() error {
	return hqgoerrors.New("cannot read config file")
}

func main() {
	if err := loadConfig(); err != nil {
		err = hqgoerrors.Wrap(err, "failed to load configuration")

		fmt.Println(err.Error()) // failed to load configuration: cannot read config file
	}
}
```

### Joining Multiple Errors

Use `Join` to combine multiple errors into one. `nil` errors are dropped, and a single non-nil error is returned unchanged. The joined error unwraps to all of its members and captures a stack trace at the join point.

```go
package main

import (
	"fmt"

	hqgoerrors "github.com/hueristiq/hq-lib-errors-go"
)

func main() {
	err := hqgoerrors.Join(
		hqgoerrors.New("name is required"),
		hqgoerrors.New("age must be positive"),
	)
	if err != nil {
		fmt.Println(err.Error())
		// name is required
		// age must be positive
	}
}
```

### Types & Structured Fields

Classify errors and attach structured data at creation with the `WithType` and `WithField` options:

```go
err := hqgoerrors.New("payment declined",
	hqgoerrors.WithType("PaymentError"),
	hqgoerrors.WithField("order_id", 1234),
	hqgoerrors.WithField("amount", 49.95),
)
```

Errors created by this package satisfy the `hqgoerrors.Error` interface. Recover it from an opaque `error` with `As` (or a type assertion) to read the type and fields:

```go
var e hqgoerrors.Error

if hqgoerrors.As(err, &e) {
	fmt.Println("Type:", e.Type())     // PaymentError
	fmt.Println("Fields:", e.Fields()) // map[amount:49.95 order_id:1234]
}
```

`Fields()` returns a snapshot copy, so it is safe to read while the error is mutated concurrently, and mutating the returned map does not affect the error. The type and fields can also be set after creation through the `MutableError` interface — `SetType(...)` and `SetField(...)` return the error for chaining — though setting them at creation time with `WithType`/`WithField` is preferred.

### Unwrapping, `Is`, `As`, and `Cause`

These mirror the standard library and traverse the whole chain, including the branches of a joined error.

- **Unwrap** the next error in the chain:

	```go
	next := hqgoerrors.Unwrap(err)
	```

- **`Is`** reports whether a sentinel appears anywhere in the chain:

	```go
	if hqgoerrors.Is(err, ErrNotFound) {
		// ...
	}
	```

	Note that errors created by this package compare by value (message and type), not identity: two separately created errors with the same message match each other. Create sentinels once and reuse them when identity matters.

- **`As`** finds the first error in the chain assignable to the target and assigns it:

	```go
	var e hqgoerrors.Error

	if hqgoerrors.As(err, &e) {
		fmt.Println("Type:", e.Type())
	}
	```

- **`Cause`** returns the deepest error in the chain (the original root cause):

	```go
	cause := hqgoerrors.Cause(err)
	```

### Formatting Errors

`ToString`, `ToJSON`, and `ToJSONString` render an error chain. Stack traces are **omitted by default**; enable them with the `FormatterWithTrace` option. Package errors also implement `fmt.Formatter`, so `fmt.Printf("%+v", err)` prints the full chain with traces. For finer control over ordering, indentation, and trace inclusion, build a `Formatter` with `NewFormatter`.

#### ... to String

```go
package main

import (
	"fmt"

	hqgoerrors "github.com/hueristiq/hq-lib-errors-go"
)

func main() {
	err := hqgoerrors.New("root error example!", hqgoerrors.WithType("ERROR_TYPE"), hqgoerrors.WithField("FIELD_KEY_1", "FIELD_VALUE_1"), hqgoerrors.WithField("FIELD_KEY_2", "FIELD_VALUE_2"))

	err = hqgoerrors.Wrap(err, "wrap error example 1!")
	err = hqgoerrors.Wrap(err, "wrap error example 2!", hqgoerrors.WithType("ERROR_TYPE_2"), hqgoerrors.WithField("FIELD_KEY_1", "FIELD_VALUE_1"), hqgoerrors.WithField("FIELD_KEY_2", "FIELD_VALUE_2"))

	formattedStr := hqgoerrors.ToString(err, hqgoerrors.FormatterWithTrace())

	fmt.Println(formattedStr)
}
```

output:

```
[ERROR_TYPE_2] wrap error example 2!

Fields:
  FIELD_KEY_1: FIELD_VALUE_1
  FIELD_KEY_2: FIELD_VALUE_2

wrap Trace:
  main.main (/home/.../main.go:13)

wrap error example 1!

wrap Trace:
  main.main (/home/.../main.go:12)

[ERROR_TYPE] root error example!

Fields:
  FIELD_KEY_1: FIELD_VALUE_1
  FIELD_KEY_2: FIELD_VALUE_2

root Trace:
  main.main (/home/.../main.go:10)
```

#### ... to JSON

`ToJSON` returns a `map[string]any` ready for `json.Marshal`; `ToJSONString` marshals it for you with indentation.

```go
package main

import (
	"fmt"

	hqgoerrors "github.com/hueristiq/hq-lib-errors-go"
)

func main() {
	err := hqgoerrors.New("root error example!", hqgoerrors.WithType("ERROR_TYPE"), hqgoerrors.WithField("FIELD_KEY_1", "FIELD_VALUE_1"), hqgoerrors.WithField("FIELD_KEY_2", "FIELD_VALUE_2"))

	err = hqgoerrors.Wrap(err, "wrap error example 1!")
	err = hqgoerrors.Wrap(err, "wrap error example 2!", hqgoerrors.WithType("ERROR_TYPE_2"), hqgoerrors.WithField("FIELD_KEY_1", "FIELD_VALUE_1"), hqgoerrors.WithField("FIELD_KEY_2", "FIELD_VALUE_2"))

	formattedJSON := hqgoerrors.ToJSONString(err, hqgoerrors.FormatterWithTrace())

	fmt.Println(formattedJSON)
}
```

output:

```json
{
  "chain": [
    {
      "fields": {
        "FIELD_KEY_1": "FIELD_VALUE_1",
        "FIELD_KEY_2": "FIELD_VALUE_2"
      },
      "message": "wrap error example 2!",
      "stack": [
        {
          "file": "/home/.../main.go",
          "function": "main.main",
          "line": 13
        }
      ],
      "type": "ERROR_TYPE_2"
    },
    {
      "message": "wrap error example 1!",
      "stack": [
        {
          "file": "/home/.../main.go",
          "function": "main.main",
          "line": 12
        }
      ]
    }
  ],
  "root": {
    "fields": {
      "FIELD_KEY_1": "FIELD_VALUE_1",
      "FIELD_KEY_2": "FIELD_VALUE_2"
    },
    "message": "root error example!",
    "stack": [
      {
        "file": "/home/.../main.go",
        "function": "main.main",
        "line": 10
      }
    ],
    "type": "ERROR_TYPE"
  }
}
```

## Contributing

Contributions are welcome and encouraged! Feel free to submit [Pull Requests](https://github.com/hueristiq/hq-lib-errors-go/pulls) or report [Issues](https://github.com/hueristiq/hq-lib-errors-go/issues). For more details, check out the [contribution guidelines](https://github.com/hueristiq/hq-lib-errors-go/blob/main/CONTRIBUTING.md).

A big thank you to all the [contributors](https://github.com/hueristiq/hq-lib-errors-go/graphs/contributors) for your ongoing support!

![contributors](https://contrib.rocks/image?repo=hueristiq/hq-lib-errors-go&max=500)

## Licensing

This package is licensed under the [MIT license](https://opensource.org/license/mit). You are free to use, modify, and distribute it, as long as you follow the terms of the license. You can find the full license text in the repository - [Full MIT license text](https://github.com/hueristiq/hq-lib-errors-go/blob/main/LICENSE).
