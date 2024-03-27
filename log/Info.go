package log

import (
	logs "github.com/sirupsen/logrus"
	"io"
	"os"
)

var logWrapper = logs.New()

func Info(msg string, track string, parent string) {
	file, err := os.OpenFile("info.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	mw := io.MultiWriter(os.Stdout, file)

	defer file.Close()

	if err == nil {
		logWrapper.Out = mw
	} else {
		logs.Info("Failed to logs to file, using default stderr")
	}

	logWrapper.SetFormatter(&logs.JSONFormatter{})

	logWrapper.WithFields(logs.Fields{
		"span":   os.Getenv("SERVICE_NAME"),
		"track":  track,
		"parent": parent,
	}).Info(msg)
}
