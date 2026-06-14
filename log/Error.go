package log

import (
	"fmt"
	"os"
	"runtime"

	logs "github.com/sirupsen/logrus"
)

func Error(msg string, track string, parent string, fields ...Field) {
	logWrapper.Out = getErrorWriter()
	logWrapper.SetFormatter(formatter)

	var fileLocation string
	_, calledFile, no, ok := runtime.Caller(1)
	if ok {
		fileLocation = fmt.Sprintf("Called from file %s, at line #%d", calledFile, no)
	}
	messageWithFileLocation := fmt.Sprintf("%s\n%s", msg, fileLocation)

	logWrapper.WithFields(applyFields(logs.Fields{
		"span":   os.Getenv("SERVICE_NAME"),
		"track":  track,
		"parent": parent,
	}, fields...)).Error(messageWithFileLocation)
}
