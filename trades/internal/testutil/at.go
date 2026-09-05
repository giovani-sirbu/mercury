package testutil

import "time"

// At parses a wall-clock stamp on the day trade 25858 emptied its budget
// (2021-07-26, UTC).
func At(clock string) time.Time {
	parsed, err := time.Parse("2006-01-02 15:04:05", "2021-07-26 "+clock)
	if err != nil {
		panic(err)
	}
	return parsed.UTC()
}
