package log

import (
	"os"

	logs "github.com/sirupsen/logrus"
)

func Warn(msg string, track string, parent string, fields ...Field) {
	logWrapper.Out = getWarnWriter()
	logWrapper.SetFormatter(formatter)

	logWrapper.WithFields(applyFields(logs.Fields{
		"span":   os.Getenv("SERVICE_NAME"),
		"track":  track,
		"parent": parent,
	}, fields...)).Warn(msg)
}
