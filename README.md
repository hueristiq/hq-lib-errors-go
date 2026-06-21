# hq-lib-errors-go

![made with go](https://img.shields.io/badge/made%20with-Go-1E90FF.svg) [![go report card](https://goreportcard.com/badge/github.com/hueristiq/hq-lib-errors-go)](https://goreportcard.com/report/github.com/hueristiq/hq-lib-errors-go) [![license](https://img.shields.io/badge/license-MIT-gray.svg?color=1E90FF)](https://github.com/hueristiq/hq-lib-errors-go/blob/master/LICENSE) ![maintenance](https://img.shields.io/badge/maintained%3F-yes-1E90FF.svg) [![open issues](https://img.shields.io/github/issues-raw/hueristiq/hq-lib-errors-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-errors-go/issues?q=is:issue+is:open) [![closed issues](https://img.shields.io/github/issues-closed-raw/hueristiq/hq-lib-errors-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-errors-go/issues?q=is:issue+is:closed) [![contribution](https://img.shields.io/badge/contributions-welcome-1E90FF.svg)](https://github.com/hueristiq/hq-lib-errors-go/blob/master/CONTRIBUTING.md)

`hq-lib-errors-go` is a [Go (Golang)](http://golang.org/) package for rich, structured error handling with full stack-trace support, error wrapping, classification, and formatting.

## Resource

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
	- [Creating Errors](#creating-errors)
	- [Wrapping Errors](#wrapping-errors)
	- [Joining Multiple Errors](#joining-multiple-errors)
	- [Structured Types & Fields](#structured-types--fields)
	- [Unwrapping, `Is`, `As`, and `Cause`](#unwrapping-is-as-and-cause)
	- [Formatting Errors](#formatting-errors)
		- [... to String](#-to-string)
		- [... to JSON](#-to-json)
- [Contributing](#contributing)
- [Licensing](#licensing)

## Features

- **Full Stack Traces:** Capture detailed call stacks at error creation and wrap points, with customizable formatting (e.g., reverse order, separators).
- **Error Chaining:** Wrap errors to add context while preserving the original stack trace and error details.
- **Error Classification:** Assign `ErrorType` values to categorize errors for programmatic handling.
- **Structured Fields:** Attach arbitrary key-value metadata (e.g., request IDs, parameters) to errors for enhanced debugging.
- **Multi-Error Support:** Join multiple errors into a single error object with a shared stack trace.
- **Flexible Formatting:** Render errors as human-readable strings or JSON-like maps, with options to include/exclude stack traces, invert chain order, or handle external errors.
- **Standards-Compliant:** Implements Go’s standard `error`, `Unwrap`, `Is`, and `As` interfaces, plus additional helpers like `Cause` for root cause analysis.

## Installation

To install `hq-lib-errors-go`, run the following command in your Go project:

```bash
go get -v -u github.com/hueristiq/hq-lib-errors-go
```

Make sure your Go environment is set up properly (Go 1.13 or later is recommended).

## Usage

### Creating Errors

Use `New` to create a root error with a full call stack captured at the point of invocation.

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

Use `Wrap` to add context to an existing error, capturing a single stack frame at the wrap point while preserving the original error’s stack trace.

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

		fmt.Println(err.Error())
	}
}
```

### Joining Multiple Errors

Use `Join` to combine multiple errors into a single error object, capturing a stack trace at the join point.

```go
package main

import (
	"fmt"

	hqgoerrors "github.com/hueristiq/hq-lib-errors-go"
)

func task1() error {
	return hqgoerrors.New("task 1 failed")
}

func task2() error {
	return hqgoerrors.New("task 2 failed")
}

func main() {
	err1 := task1()
	err2 := task2()

	err := hqgoerrors.Join(err1, err2)
	if err != nil {
		fmt.Println(err.Error())
	}
}
```

### Structured Types & Fields

You can classify errors and attach structured data:

```go
err := hqgoerrors.New("payment declined",
	hqgoerrors.WithType("PaymentError"),
	hqgoerrors.WithField("order_id", 1234),
	hqgoerrors.WithField("amount", 49.95),
)
```

- Retrieve:

	```go
	if e, ok := err.(hqgoerrors.Error); ok {
		fmt.Println("Type:", e.Type())
		fmt.Println("Fields:", e.Fields())
	}
	```

### Unwrapping, `Is`, `As`, and `Cause`

- Standard Unwrap:

	```go
	next := hqgoerrors.Unwrap(err)
	```

- Deep equality:

	```go
	if hqgoerrors.Is(err, targetErr) { … }
	```

- Type assertion:

	```go
	var myErr *hqgoerrors.Error

	if hqgoerrors.As(err, &myErr) {
		fmt.Println("Got:", myErr.Msg)
	}
	```

- Root cause:

	```go
	cause := hqgoerrors.Cause(err)
	```

### Formatting Errors

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

	formattedStr := hqgoerrors.ToString(err, hqgoerrors.FormatWithTrace())

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
  main.main (/home/.../hq-lib-errors-go/examples/basic/main.go:13)

wrap error example 1!

wrap Trace:
  main.main (/home/.../hq-lib-errors-go/examples/basic/main.go:12)

[ERROR_TYPE] root error example!

Fields:
  FIELD_KEY_1: FIELD_VALUE_1
  FIELD_KEY_2: FIELD_VALUE_2

root Trace:
  main.main (/home/.../hq-lib-errors-go/examples/basic/main.go:10)
  main.main (/home/.../hq-lib-errors-go/examples/basic/main.go:12)
  main.main (/home/.../hq-lib-errors-go/examples/basic/main.go:13)
```

#### ... to JSON

```go
package main

import (
	"encoding/json"
	"fmt"

	hqgoerrors "github.com/hueristiq/hq-lib-errors-go"
)

func main() {
	err := hqgoerrors.New("root error example!", hqgoerrors.WithType("ERROR_TYPE"), hqgoerrors.WithField("FIELD_KEY_1", "FIELD_VALUE_1"), hqgoerrors.WithField("FIELD_KEY_2", "FIELD_VALUE_2"))

	err = hqgoerrors.Wrap(err, "wrap error example 1!")
	err = hqgoerrors.Wrap(err, "wrap error example 2!", hqgoerrors.WithType("ERROR_TYPE_2"), hqgoerrors.WithField("FIELD_KEY_1", "FIELD_VALUE_1"), hqgoerrors.WithField("FIELD_KEY_2", "FIELD_VALUE_2"))

	formattedJSON := hqgoerrors.ToJSON(err, hqgoerrors.FormatWithTrace())

	bytes, _ := json.Marshal(formattedJSON)

	fmt.Println(string(bytes))
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
          "file": "/home/.../hq-lib-errors-go/examples/basic/main.go",
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
          "file": "/home/.../hq-lib-errors-go/examples/basic/main.go",
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
        "file": "/home/.../hq-lib-errors-go/examples/basic/main.go",
        "function": "main.main",
        "line": 10
      },
      {
        "file": "/home/.../hq-lib-errors-go/examples/basic/main.go",
        "function": "main.main",
        "line": 12
      },
      {
        "file": "/home/.../hq-lib-errors-go/examples/basic/main.go",
        "function": "main.main",
        "line": 13
      }
    ],
    "type": "ERROR_TYPE"
  }
}
```

## Contributing

Contributions are welcome and encouraged! Feel free to submit [Pull Requests](https://github.com/hueristiq/hq-lib-errors-go/pulls) or report [Issues](https://github.com/hueristiq/hq-lib-errors-go/issues). For more details, check out the [contribution guidelines](https://github.com/hueristiq/hq-lib-errors-go/blob/master/CONTRIBUTING.md).

A big thank you to all the [contributors](https://github.com/hueristiq/hq-lib-errors-go/graphs/contributors) for your ongoing support!

![contributors](https://contrib.rocks/image?repo=hueristiq/hq-lib-errors-go&max=500)

## Licensing

This package is licensed under the [MIT license](https://opensource.org/license/mit). You are free to use, modify, and distribute it, as long as you follow the terms of the license. You can find the full license text in the repository - [Full MIT license text](https://github.com/hueristiq/hq-lib-errors-go/blob/master/LICENSE).