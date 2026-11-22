# Go Framework Project

[English](README.md) | [中文](README.zh.md)

This is a lightweight development framework based on the Go language, suitable for rapidly building efficient and maintainable applications. The project is divided into multiple modules, including core functionalities, code generation tools, and components related to web development.

---

## Key Features

- **Logging System**: Built on the standard library `log/slog`, with stdlib text output and lumberjack file rotation; use `slog.Info/Debug/...` directly after initialization. Includes a gin request logging middleware.
- **Response Wrapping**: Provides a unified response structure, supporting formats for both success and failure returns.
- **Pagination Support**: Offers data structures and response encapsulation for pagination in web applications.
- **Code Generation**: Includes a code generation tool that can automatically generate processing logic templates.

---

## Module Description

- **core**: Core library, containing the logging module `slogx`.
- **gen**: Code generation module, including the template engine and generation logic.
- **web**: Web development support module, containing pagination structures, unified response formats, and gin middleware.

---

## Installation and Usage

### Initialize the Logging Module

```go
import (
    "log/slog"

    "gitee.com/luogeng_cn/go-framework/core/slogx"
)

func main() {
    // level <= slog.LevelDebug: terminal output only (debug mode);
    // otherwise: terminal + log file (default ./logs/app.log, overridable via the second argument)
    slogx.InitDefault(slog.LevelInfo)

    slog.Info("Service started successfully")
}
```

### Gin Request Logging Middleware

```go
import (
    "github.com/gin-gonic/gin"

    "gitee.com/luogeng_cn/go-framework/web/gin/middleware"
)

func newEngine() *gin.Engine {
    r := gin.New()
    r.Use(middleware.Logger(), middleware.Recovery())
    return r
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