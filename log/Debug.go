package log

import (
	logs "github.com/sirupsen/logrus"
	"os"
)

func Debug(msg ...any) {
	if os.Getenv("DEBUG") == "true" {
		logWrapper.SetFormatter(&logs.JSONFormatter{})

		for _, arg := range msg {
			logWrapper.Debug(arg)
		}
	}
}
