# Go Framework 项目

这是一个基于 Go 语言的轻量级开发框架，适用于快速构建高效、可维护的应用程序。该项目分为多个模块，包括核心功能、代码生成工具以及 Web 开发相关组件。

---

## 主要功能

- **日志系统**：使用 `zap` 提供高性能日志记录功能，并封装了常用的日志输出方法。
- **响应封装**：提供统一的响应结构，支持成功与失败的返回格式。
- **分页支持**：为 Web 应用提供分页数据结构和响应封装。
- **代码生成**：包含代码生成工具，可自动生成处理逻辑模板。

---

## 模块说明

- **core**：核心库，包含日志模块 `logger`。
- **gen**：代码生成模块，包含模板引擎和生成逻辑。
- **web**：Web 开发支持模块，包含分页结构和统一响应格式。

---

## 安装与使用

### 初始化日志模块

```go
import "core/logger"

func main() {
    if err := logger.Init(); err != nil {
        panic(err)
    }

    logger.Infof("服务启动成功")
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