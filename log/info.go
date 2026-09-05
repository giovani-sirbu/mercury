package log

import (
	"os"

	logs "github.com/sirupsen/logrus"
)

var logWrapper = logs.New()

func Info(msg string, track string, parent string, fields ...Field) {
	logWrapper.Out = getInfoWriter()
	logWrapper.SetFormatter(formatter)

	logWrapper.WithFields(applyFields(logs.Fields{
		"span":   os.Getenv("SERVICE_NAME"),
		"track":  track,
		"parent": parent,
	}, fields...)).Info(msg)
}
