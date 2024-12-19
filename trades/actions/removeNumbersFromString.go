package actions

import (
	"regexp"
)

func RemoveNumbersFromString(str string) string {
	re := regexp.MustCompile(`\d`)
	output := re.ReplaceAllString(str, "")

	return output
}
