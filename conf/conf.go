package conf

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"os"
	"path"
)

func NewLogger() *zap.SugaredLogger {
	//cfg := zap.NewProductionConfig()
	cfg := zap.NewDevelopmentConfig()
	logDir := viper.GetString("logDir")
	if logDir != "" {
		if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
			panic(err)
		}
		cfg.OutputPaths = []string{path.Join(logDir, "log.log")}
	}
	l, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return l.Sugar()
}
