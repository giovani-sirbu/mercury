package aggragates

import (
	"reflect"
	"testing"
)

// MessageKey is a varchar(64) indexed column the wire carries as messageKey, beside the message.
func TestTradesLogsCarryTheMessageKeyColumn(t *testing.T) {
	field, ok := reflect.TypeOf(TradesLogs{}).FieldByName("MessageKey")
	if !ok || field.Type.Kind() != reflect.String {
		t.Fatal("TradesLogs must carry a string MessageKey")
	}
	if gorm := field.Tag.Get("gorm"); gorm != "type:varchar(64);index" {
		t.Errorf("gorm tag = %q, want type:varchar(64);index", gorm)
	}
	if json := field.Tag.Get("json"); json != "messageKey" {
		t.Errorf("json tag = %q, want messageKey", json)
	}
	if message, _ := reflect.TypeOf(TradesLogs{}).FieldByName("Message"); message.Tag.Get("json") != "message" {
		t.Errorf("the message keeps its own json tag, got %q", message.Tag.Get("json"))
	}
}
