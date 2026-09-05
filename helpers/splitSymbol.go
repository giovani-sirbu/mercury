package helpers

import "strings"

// SplitSymbol splits a "BASE/QUOTE" trading pair into its base and quote
// assets. Both are empty when the symbol does not contain exactly one slash.
func SplitSymbol(symbol string) (string, string) {
	parts := strings.Split(symbol, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
