package slog

import (
	"io"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

func InitDefault() {
	// 文件切割配置
	fileWriter := &lumberjack.Logger{
		Filename:   "./logs/app.log", // 日志文件
		MaxSize:    100,              // MB
		MaxBackups: 10,               // 保留旧文件
		MaxAge:     30,               // 天
		Compress:   true,             // 压缩
	}

	// handler
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		AddSource: true,
	}

	if os.Getenv(gin.EnvGinMode) == gin.ReleaseMode {
		// if gin.Mode() == gin.ReleaseMode {

		// 同时输出到控制台和文件
		writer := io.MultiWriter(os.Stdout, fileWriter)
		opts.Level = slog.LevelInfo
		handler = slog.NewJSONHandler(writer, opts)
		// handler = slog.NewTextHandler(writer, opts)
		// handler = NewColorHandler(writer, opts)
	} else {

		// 同时输出到控制台和文件
		writer := io.MultiWriter(os.Stdout)
		opts.Level = slog.LevelDebug
		// handler = slog.NewTextHandler(writer, opts)
		handler = NewColorHandler(writer, opts)
	}

	logger := slog.New(handler)

	// 设置为全局默认 logger
	slog.SetDefault(logger)
}
