package logger

import (
	"os"

	"go.uber.org/zap"
)

var log *zap.SugaredLogger

var (
	Errorf func(format string, args ...any)
	Warnf  func(format string, args ...any)
	Infof  func(format string, args ...any)
	Debugf func(format string, args ...any)
)

func Init() (err error) {
	mode := os.Getenv("GIN_MODE")
	var _log *zap.Logger
	// zap.Config
	if mode == "release" {
		// 生产环境使用 zap.NewProduction()，
		_log, err = zap.NewProduction()
	} else {
		// 开发环境使用 zap.NewDevelopment()，
		_log, err = zap.NewDevelopment()
	}
	if err != nil {
		return err
	}
	log = _log.Sugar()

	Errorf = log.Errorf
	Warnf = log.Warnf
	Infof = log.Infof
	Debugf = log.Debugf
	return nil
}
