package log

import (
	"fmt"
	"os"
)

func Debug(msg ...any) {
	if os.Getenv("DEBUG") == "true" {
		fmt.Println(msg)
		logWrapper.Debug(msg)

		for _, arg := range msg {
			logWrapper.Debug(arg)
		}
	}
}
