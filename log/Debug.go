package log

import (
	"fmt"
	"os"
)

func Debug(msg any) {
	if os.Getenv("DEBUG") == "true" {
		fmt.Println(msg)
	}
}
