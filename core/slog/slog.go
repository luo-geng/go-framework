package slog

import (
	"io"
	"log/slog"
	"os"
	"time"

	"charm.land/log/v2"
	"gopkg.in/natefinch/lumberjack.v2"
)

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

	var handler slog.Handler

	if level <= slog.LevelDebug {
		// debug 模式：只输出到控制台
		handler = log.NewWithOptions(os.Stderr, log.Options{
			// ReportCaller:    true,
			ReportTimestamp: true,
			TimeFormat:      time.DateTime,
		})
	} else {
		// 正常模式：同时输出到控制台和文件
		writer := io.MultiWriter(os.Stdout, fileWriter)
		handler = log.NewWithOptions(writer, log.Options{
			ReportTimestamp: true,
			TimeFormat:      time.DateTime,
		})
	}

	logger := slog.New(handler)

	// 设置为全局默认 logger
	slog.SetDefault(logger)
}
