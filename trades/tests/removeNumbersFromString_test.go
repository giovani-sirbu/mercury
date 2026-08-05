package tests

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
)

func TestRemoveNumbersFromString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"Mixed", "price 123.45 error", "price . error"},
		{"NoNumbers", "hello world", "hello world"},
		{"OnlyNumbers", "12345", ""},
		{"Empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := actions.RemoveNumbersFromString(tt.input)
			if got != tt.want {
				t.Errorf("RemoveNumbersFromString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
