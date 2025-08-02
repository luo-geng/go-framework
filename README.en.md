# Go Framework Project

This is a lightweight development framework based on the Go language, suitable for rapidly building efficient and maintainable applications. The project is divided into multiple modules, including core functionalities, code generation tools, and components related to web development.

---

## Key Features

- **Logging System**: Uses `zap` to provide high-performance logging capabilities, and encapsulates commonly used logging methods.
- **Response Wrapping**: Provides a unified response structure, supporting formats for both success and failure returns.
- **Pagination Support**: Offers data structures and response encapsulation for pagination in web applications.
- **Code Generation**: Includes a code generation tool that can automatically generate processing logic templates.

---

## Module Description

- **core**: Core library, containing the logging module `logger`.
- **gen**: Code generation module, including the template engine and generation logic.
- **web**: Web development support module, containing pagination structures and unified response formats.

---

## Installation and Usage

### Initialize the Logging Module

```go
import "core/logger"

func main() {
    if err := logger.Init(); err != nil {
        panic(err)
    }

    logger.Infof("Service started successfully")
}
```

### Using the Unified Response Structure

```go
import "web/resp"

func exampleHandler() {
    resp := resp.Success("Operation succeeded")
    // Or return an error message
    // resp := resp.Fail(500, "An error occurred")
}
```

### Pagination Functionality

```go
import "web"

func paginationExample() {
    page := &web.Pagination{
        Page:  1,
        Limit: 10,
    }

    // Use PageResp to return paginated data
    pageResp := &web.PageResp[string]{
        List:     []string{"item1", "item2"},
        Page:     1,
        PageSize: 10,
        Total:    2,
    }
}
```

---

## Contribution Guide

Contributions to code and documentation improvements are welcome. Please follow these steps:

1. Fork this repository
2. Create a new branch (`git checkout -b feature/new-feature`)
3. Commit your changes (`git commit -am 'Add new feature'`)
4. Push the branch (`git push origin feature/new-feature`)
5. Create a Pull Request

---

## License

This project uses the MIT License. See the [LICENSE](LICENSE) file for details.