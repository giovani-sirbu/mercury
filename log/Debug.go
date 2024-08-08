package log

import (
	logs "github.com/sirupsen/logrus"
	"os"
)

func Debug(msg interface{}) {
	if os.Getenv("DEBUG") == "true" {
		logWrapper.SetFormatter(&logs.JSONFormatter{})
		logWrapper.Debug(msg)
	}
}
