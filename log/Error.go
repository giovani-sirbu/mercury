package log

import (
	"fmt"
	"io"
	"os"
	"runtime"

	logs "github.com/sirupsen/logrus"
)

func Error(msg string, track string, parent string) {
	file, err := os.OpenFile("error.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	mw := io.MultiWriter(os.Stdout, file)

	defer file.Close()

	if err == nil {
		logWrapper.Out = mw
	} else {
		logs.Info("Failed to log to file, using default stderr")
	}

	logWrapper.SetFormatter(&logs.JSONFormatter{})

	var fileLocation string

	_, calledFile, no, ok := runtime.Caller(1)
	if ok {
		fileLocation = fmt.Sprintf("Called from file %s, at line #%d", calledFile, no)
	}

	messageWithFileLocation := fmt.Sprintf("%s\n%s", msg, fileLocation)

	logWrapper.WithFields(logs.Fields{
		"span":   os.Getenv("SERVICE_NAME"),
		"track":  track,
		"parent": parent,
	}).Error(messageWithFileLocation)
}
