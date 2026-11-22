package slogx

import (
	"io"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// InitDefault 初始化全局 slog 默认 logger。
//
// level 决定输出级别过滤：
//   - level <= LevelDebug：仅输出到终端（开发调试），不写文件
//   - 其他：终端 + 日志文件同时输出
//
// 终端与文件统一使用标准库 slog TextHandler（零额外依赖，避免体积膨胀）。
// log_path 可覆盖默认日志文件路径 ./logs/app.log。
func InitDefault(level slog.Level, log_path ...string) {
	filename := "./logs/app.log"
	if len(log_path) > 0 {
		filename = log_path[0]
	}

	// 文件切割配置
	fileWriter := &lumberjack.Logger{
		Filename:   filename, // 日志文件
		MaxSize:    100,      // MB
		MaxBackups: 10,       // 保留旧文件
		MaxAge:     30,       // 天
		Compress:   true,     // 压缩
	}

	var writer io.Writer = os.Stderr
	if level > slog.LevelDebug {
		// 非 debug 模式：同时输出到终端和文件
		writer = io.MultiWriter(os.Stdout, fileWriter)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: level,
	})))
}
