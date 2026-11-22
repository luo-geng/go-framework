# Go Framework 项目

[English](README.md) | [中文](README.zh.md)

这是一个基于 Go 语言的轻量级开发框架，适用于快速构建高效、可维护的应用程序。该项目分为多个模块，包括核心功能、代码生成工具以及 Web 开发相关组件。

---

## 主要功能

- **日志系统**：基于标准库 `log/slog`，零额外依赖（仅 lumberjack 文件切割），初始化后直接使用 `slog.Info/Debug/...`；提供 gin 请求日志中间件。
- **响应封装**：提供统一的响应结构，支持成功与失败的返回格式。
- **分页支持**：为 Web 应用提供分页数据结构和响应封装。
- **代码生成**：包含代码生成工具，可自动生成处理逻辑模板。

---

## 模块说明

- **core**：核心库，包含日志模块 `slogx`。
- **gen**：代码生成模块，包含模板引擎和生成逻辑。
- **web**：Web 开发支持模块，包含分页结构、统一响应格式和 gin 中间件。

---

## 安装与使用

### 初始化日志模块

```go
import (
    "log/slog"

    "gitee.com/luogeng_cn/go-framework/core/slogx"
)

func main() {
    // level <= slog.LevelDebug 时仅输出到终端（调试模式）；
    // 否则终端 + 日志文件同时输出（默认 ./logs/app.log，可用第二个参数覆盖）
    slogx.InitDefault(slog.LevelInfo)

    slog.Info("服务启动成功")
}
```

### gin 请求日志中间件

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

### 使用统一响应结构

```go
import "web/resp"

func exampleHandler() {
    resp := resp.Success("操作成功")
    // 或者返回错误信息
    // resp := resp.Fail(500, "发生错误")
}
```

### 分页功能

```go
import "web"

func paginationExample() {
    page := &web.Pagination{
        Page:  1,
        Limit: 10,
    }

    // 使用 PageResp 返回分页数据
    pageResp := &web.PageResp[string]{
        List:     []string{"item1", "item2"},
        Page:     1,
        PageSize: 10,
        Total:    2,
    }
}
```

---

## 贡献指南

欢迎贡献代码和改进文档。请遵循以下步骤：

1. Fork 本仓库
2. 创建新分支 (`git checkout -b feature/new-feature`)
3. 提交更改 (`git commit -am 'Add new feature'`)
4. 推送分支 (`git push origin feature/new-feature`)
5. 创建 Pull Request

---

## 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。