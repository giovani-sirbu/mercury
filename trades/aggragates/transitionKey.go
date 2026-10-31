package aggragates

import (
	"strings"
	"unicode"
)

// UpperSnake is a camelCase state in upper snake case ("trailingBuy" → "TRAILING_BUY"); "" stays "".
func UpperSnake(word string) string {
	var snake strings.Builder
	for index, letter := range word {
		if index > 0 && unicode.IsUpper(letter) {
			snake.WriteByte('_')
		}
		snake.WriteRune(unicode.ToUpper(letter))
	}
	return snake.String()
}

// TransitionKey is the messageKey of a transition row: the old state and the new one in upper snake case ("BUY_TO_TRAILING_BUY").
func TransitionKey(oldPosition, newPosition string) string {
	return UpperSnake(oldPosition) + "_TO_" + UpperSnake(newPosition)
}
