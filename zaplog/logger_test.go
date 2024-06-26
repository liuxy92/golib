package zaplog

import (
	"fmt"
	"go.uber.org/zap"
	"testing"
	"time"
)

func TestZaplog(t *testing.T) {
	data := &Options{
		LogLevel:   "info",
		LogFileDir: "../logs",
		AppName:    "logtool",
		MaxSize:    30,
		MaxBackups: 7,
		MaxAge:     7,
		CutType:    1,
		Config:     zap.Config{},
	}
	data.Development = true
	InitLogger(data)
	for i := 0; i < 2; i++ {
		time.Sleep(1 * time.Second)
		logger.Debug(fmt.Sprint("debug log ", i), zap.Int("line", 999))
		logger.Info(fmt.Sprint("Info log ", i), zap.Any("level", "1231231231"))
		logger.Warn(fmt.Sprint("warn log ", i), zap.String("level", `{"a":"4","b":"5"}`))
		logger.Error(fmt.Sprint("err log ", i), zap.String("level", `{"a":"7","b":"8"}`))
	}
}
