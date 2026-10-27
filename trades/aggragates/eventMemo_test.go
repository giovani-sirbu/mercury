package aggragates

import (
	"encoding/json"
	"testing"
)

type memoDoc struct {
	Event string   `json:"event"`
	Tags  []string `json:"tags,omitempty"`
}

// withMemoizedEvents turns the memos on for one test.
func withMemoizedEvents(t *testing.T) {
	t.Helper()
	MemoizeEvents(true)
	t.Cleanup(func() { MemoizeEvents(false) })
}

func TestEventMemo_OffUnlessTurnedOn(t *testing.T) {
	var memo EventMemo[memoDoc]

	for _, event := range []string{"held", "sold"} {
		got, err := memo.Decode(TradesStrategyEvents{ID: 7, Data: json.RawMessage(`{"event":"` + event + `"}`)})
		if err != nil || got.Event != event {
			t.Fatalf("decode of %q with the memo off = %+v, %v", event, got, err)
		}
	}
}

func TestEventMemo_StoredEventDecodedOnce(t *testing.T) {
	withMemoizedEvents(t)

	var memo EventMemo[memoDoc]
	first := TradesStrategyEvents{ID: 7, Data: json.RawMessage(`{"event":"held","tags":["a"]}`)}

	got, err := memo.Decode(first)
	if err != nil || got.Event != "held" || len(got.Tags) != 1 {
		t.Fatalf("first decode = %+v, %v", got, err)
	}

	// Same id and size, another document: the remembered one answers.
	again, err := memo.Decode(TradesStrategyEvents{ID: 7, Data: json.RawMessage(`{"event":"sold","tags":["b"]}`)})
	if err != nil || again.Event != "held" {
		t.Fatalf("second decode = %+v, %v; want the remembered document", again, err)
	}
}

func TestEventMemo_UnstoredEventNeverRemembered(t *testing.T) {
	withMemoizedEvents(t)

	var memo EventMemo[memoDoc]

	for _, event := range []string{"held", "sold"} {
		got, err := memo.Decode(TradesStrategyEvents{Data: json.RawMessage(`{"event":"` + event + `"}`)})
		if err != nil || got.Event != event {
			t.Fatalf("decode of %q = %+v, %v", event, got, err)
		}
	}
}

func TestEventMemo_SizeTellsReusedIdsApart(t *testing.T) {
	withMemoizedEvents(t)

	var memo EventMemo[memoDoc]

	if _, err := memo.Decode(TradesStrategyEvents{ID: 3, Data: json.RawMessage(`{"event":"held"}`)}); err != nil {
		t.Fatal(err)
	}

	got, err := memo.Decode(TradesStrategyEvents{ID: 3, Data: json.RawMessage(`{"event":"activated"}`)})
	if err != nil || got.Event != "activated" {
		t.Fatalf("decode of the longer document = %+v, %v", got, err)
	}
}

func TestEventMemo_ErrorRemembered(t *testing.T) {
	withMemoizedEvents(t)

	var memo EventMemo[memoDoc]
	broken := TradesStrategyEvents{ID: 9, Data: json.RawMessage(`{"event":`)}

	if _, err := memo.Decode(broken); err == nil {
		t.Fatal("broken document decoded")
	}

	if _, err := memo.Decode(broken); err == nil {
		t.Fatal("broken document decoded on the remembered read")
	}
}
