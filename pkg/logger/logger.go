package logger

import (
	"go.uber.org/zap"
)

var Log *zap.Logger

func InitLogger() {
	var err error
	// In production, use NewProduction()
	Log, err = zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
}

func Sync() {
	_ = Log.Sync()
}
