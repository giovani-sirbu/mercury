package actions

import "strings"

func extractExchangeErrorMessage(input string) string {
	const prefix = "msg="
	idx := strings.Index(input, prefix)
	if idx == -1 {
		return input
	}

	return input[idx+len(prefix):]
}
