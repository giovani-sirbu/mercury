package helpers

import (
	"regexp"
)

// RemoveNumbersFromString strips every digit, so two trade-log messages that
// differ only by amounts or ids compare equal (log de-duplication).
func RemoveNumbersFromString(str string) string {
	re := regexp.MustCompile(`\d`)
	output := re.ReplaceAllString(str, "")

	return output
}
