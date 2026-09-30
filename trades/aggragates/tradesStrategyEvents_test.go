package aggragates_test

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/cache/v9"
	"gorm.io/gorm/schema"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/gates/smarttakeloss"
)

// The storage contract of the strategy events: what a trade's events look
// like on the three wires they travel (JSON between agora and hermes, msgpack
// in the Dragonfly cache, jsonb in Postgres), what NewStrategyEvent promises
// about the document it stores, and the vocabulary every writer files an event
// under. Black-box, on the exported API, and with the gate packages' own
// constructors: a fixture never hand-builds what production writes.

// eventsClock is the clock the fixtures share.
var eventsClock = time.Date(2026, time.September, 30, 9, 0, 0, 0, time.UTC)

func clockAt(minutes int) time.Time {
	return eventsClock.Add(time.Duration(minutes) * time.Minute)
}

// filed is one writer's event with the (param, gate, kind) it must be filed
// under. The literals are the stored contract: the rows already in a database
// carry them, and a rename would orphan every one.
type filed struct {
	name              string
	event             aggragates.TradesStrategyEvents
	param, gate, kind string
}

// everyWriter is one event from every exported constructor of the gate
// packages, and from every row builder of the smart take loss, on the trade.
// The reasons carry the characters a JSON encoder escapes.
func everyWriter(trade aggragates.Trades, at time.Time) []filed {
	reasons := []string{"leg down 7.0% from its high close", `vol > 2x & <quiet> "bars" \ 日本`}
	fromRow := func(row smarttakeloss.Row) aggragates.TradesStrategyEvents {
		_, event := smarttakeloss.Rows(trade, row, at)
		return event
	}
	fromOpened := func(opened dynamicparams.Opened) aggragates.TradesStrategyEvents {
		_, event := opened.Rows(trade, 100, at)
		return event
	}

	return []filed{
		{"first fill activated", cooldown.NewFirstFillEvent(trade.ID, cooldown.FirstFillEvent{Event: cooldown.FirstFillActivated, Price: 100.125, Reference: 100.125}, at), "cooldown", "firstFill", "activated"},
		{"first fill armed", cooldown.NewFirstFillEvent(trade.ID, cooldown.FirstFillEvent{Event: cooldown.FirstFillArmed, Price: 97.4, Reference: 100.125, Anchor: 97.4}, at), "cooldown", "firstFill", "armed"},
		{"first fill entered", cooldown.NewFirstFillEvent(trade.ID, cooldown.FirstFillEvent{Event: cooldown.FirstFillEntered, Price: 102.7, Reference: 100.125}, at), "cooldown", "firstFill", "entered"},
		{"first fill verdict held", cooldown.NewFirstFillEvent(trade.ID, cooldown.FirstFillEvent{Event: cooldown.FirstFillVerdictHeld}, at), "cooldown", "firstFill", "verdictHeld"},
		{"depth priority held", cooldown.NewDepthPriorityEvent(trade.ID, cooldown.DepthPriorityEvent{Event: gates.EventHeld, PrioritySymbol: "ETH/USDT", PriorityDepth: 5, PriorityMaxDepth: 8, Depth: 2, MaxDepth: 8}, at), "cooldown", "depthPriority", "held"},
		{"depth spacing held", cooldown.NewDepthSpacingEvent(trade.ID, cooldown.DepthSpacingEvent{Event: gates.EventHeld, Depth: 3, Step: 2, Hold: 2 * time.Hour, Release: 97.4}, at), "cooldown", "depthSpacing", "held"},
		{"slow decline pending row", fromRow(smarttakeloss.PendingRow("buy", 184.45, reasons)), "smartTakeLoss", "slowDecline", "pending"},
		{"slow decline cancelled row", fromRow(smarttakeloss.CancelledRow("buy", 179.78, reasons)), "smartTakeLoss", "slowDecline", "cancelled"},
		{"slow decline reset row", fromRow(smarttakeloss.ResetRow("stopLoss", 179.78)), "smartTakeLoss", "slowDecline", "reset"},
		{"indecision latched row", fromRow(smarttakeloss.LatchedRow("buy", 175.83, reasons)), "smartTakeLoss", "indecision", "latched"},
		{"exit row", fromRow(smarttakeloss.ExitRow(trade, "a rule that sold", 175.39)), "smartTakeLoss", "slowDecline", "sold"},
		{"opened", fromOpened(dynamicparams.Opened{Points: 0.4, Depths: 1}), "dynamicParams", "opened", "opened"},
		{"opened with no amount", fromOpened(dynamicparams.Opened{}), "dynamicParams", "opened", "opened"},
	}
}

// storedEvents is the events of every writer as a database hands them back:
// numbered in write order, one minute apart, every seventh with no stamp.
func storedEvents(trade aggragates.Trades, count int) []aggragates.TradesStrategyEvents {
	writers := everyWriter(trade, eventsClock)
	stored := make([]aggragates.TradesStrategyEvents, count)
	for i := range stored {
		stored[i] = writers[i%len(writers)].event
		stored[i].ID = uint(i + 1)
		stored[i].CreatedAt = clockAt(i)
		if i%7 == 6 {
			stored[i].CreatedAt = time.Time{}
		}
	}
	return stored
}

// carrying is a trade with count events and the log row beside each.
func carrying(count int) aggragates.Trades {
	trade := aggragates.Trades{ID: 7, Symbol: "LINK/USDT", PositionType: "stopLoss", PositionPrice: 184.45}
	trade.StrategyEvents = storedEvents(trade, count)
	for i, event := range trade.StrategyEvents {
		trade.Logs = append(trade.Logs, aggragates.TradesLogs{
			ID: uint(i + 1), TradeID: trade.ID, Message: fmt.Sprintf("Hold stopLoss: row %d", i), Type: aggragates.LOG_INFO,
			Price: 184.45, CreatedAt: event.CreatedAt, UpdatedAt: event.CreatedAt,
		})
	}
	return trade
}

// assertEventsEqual fails unless the two lists are the same events, field by
// field. The times are compared as instants (a codec may hand them back in
// another zone) and the document byte for byte.
func assertEventsEqual(t *testing.T, wire string, got, want []aggragates.TradesStrategyEvents) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d events came back, want %d", wire, len(got), len(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.ID != w.ID || g.TradeID != w.TradeID || g.Param != w.Param || g.Gate != w.Gate {
			t.Errorf("%s: event %d = (id %d, trade %d, %q, %q), want (id %d, trade %d, %q, %q)",
				wire, i, g.ID, g.TradeID, g.Param, g.Gate, w.ID, w.TradeID, w.Param, w.Gate)
		}
		if !bytes.Equal(g.Data, w.Data) {
			t.Errorf("%s: event %d document = %s, want %s", wire, i, g.Data, w.Data)
		}
		if !g.CreatedAt.Equal(w.CreatedAt) || g.CreatedAt.IsZero() != w.CreatedAt.IsZero() {
			t.Errorf("%s: event %d stamp = %v, want %v", wire, i, g.CreatedAt, w.CreatedAt)
		}
	}
}

func sortedKeys(object map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// The events ride inside the trade's JSON, which is how agora serves a trade
// to hermes: the field is named strategyEvents, each event carries exactly
// these keys, and its document is embedded as an object — never a string or
// base64 — so a reader on the other side finds its kind without decoding twice.
func TestTradesCarryTheirStrategyEventsThroughJSON(t *testing.T) {
	trade := carrying(len(everyWriter(aggragates.Trades{}, eventsClock)) + 3)

	encoded, err := json.Marshal(trade)
	if err != nil {
		t.Fatalf("a trade with events must marshal: %v", err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &top); err != nil {
		t.Fatal(err)
	}
	rawEvents, present := top["strategyEvents"]
	if !present {
		t.Fatalf("the trade's JSON has no strategyEvents field, keys %v", sortedKeys(top))
	}
	var objects []map[string]json.RawMessage
	if err := json.Unmarshal(rawEvents, &objects); err != nil || len(objects) != len(trade.StrategyEvents) {
		t.Fatalf("strategyEvents must be the list of the trade's events: %d of %d, %v", len(objects), len(trade.StrategyEvents), err)
	}
	wantKeys := []string{"createdAt", "data", "gate", "id", "param", "tradeId"}
	for i, object := range objects {
		if got := sortedKeys(object); !reflect.DeepEqual(got, wantKeys) {
			t.Fatalf("event %d keys = %v, want %v", i, got, wantKeys)
		}
		if !bytes.HasPrefix(object["data"], []byte("{")) {
			t.Fatalf("event %d: data must be embedded as a JSON object, got %s", i, object["data"])
		}
	}

	var decoded aggragates.Trades
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("the trade must unmarshal: %v", err)
	}
	assertEventsEqual(t, "JSON", decoded.StrategyEvents, trade.StrategyEvents)
	if len(decoded.Logs) != len(trade.Logs) || decoded.Logs[3].Message != trade.Logs[3].Message {
		t.Fatalf("the log rows travel beside the events unchanged, got %d rows", len(decoded.Logs))
	}
	for i, event := range decoded.StrategyEvents {
		if event.Kind() == "" {
			t.Errorf("event %d lost its kind on the wire: %s", i, event.Data)
		}
	}
}

// A trade published before events existed — or by an agora that drops what it
// does not know — arrives without the field or with a null, and is a trade
// with no events: not an error, and nothing for any fold to read.
func TestATradePayloadWithoutEventsDecodesToNone(t *testing.T) {
	for name, payload := range map[string]string{
		"no field":      `{"id":7,"logs":[]}`,
		"null":          `{"id":7,"logs":[],"strategyEvents":null}`,
		"an empty list": `{"id":7,"logs":[],"strategyEvents":[]}`,
	} {
		var trade aggragates.Trades
		if err := json.Unmarshal([]byte(payload), &trade); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(trade.StrategyEvents) != 0 || len(trade.StrategyEventsOf("cooldown", "firstFill")) != 0 {
			t.Errorf("%s: %d events, want none", name, len(trade.StrategyEvents))
		}
	}
}

// hermes caches the trade in Dragonfly with go-redis/cache, whose codec is
// msgpack under s2 compression. cache.New(&cache.Options{}) is that codec with
// no server behind it. The events must come back as they went in: the document
// byte for byte (it is a byte slice, not a string), the stamps as instants
// (msgpack decodes a time in the local zone), an unstamped event still
// unstamped — the fold reads an unknown clock as "never expires", so a zero
// that came back as a real instant would change a gate's behavior.
func TestTradesCarryTheirStrategyEventsThroughTheDragonflyCodec(t *testing.T) {
	codec := cache.New(&cache.Options{})
	writers := len(everyWriter(aggragates.Trades{}, eventsClock))

	for name, count := range map[string]int{"one event": 1, "one of every writer": writers, "two hundred events": 200} {
		trade := carrying(count)

		encoded, err := codec.Marshal(trade)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		var decoded aggragates.Trades
		if err := codec.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		assertEventsEqual(t, name, decoded.StrategyEvents, trade.StrategyEvents)
		if len(decoded.Logs) != len(trade.Logs) || decoded.ID != trade.ID {
			t.Fatalf("%s: the rest of the trade must come back too, got %d rows of trade %d", name, len(decoded.Logs), decoded.ID)
		}
		for i, event := range decoded.StrategyEvents {
			if event.Kind() != trade.StrategyEvents[i].Kind() {
				t.Errorf("%s: event %d kind = %q, want %q", name, i, event.Kind(), trade.StrategyEvents[i].Kind())
			}
		}

		// The same codec accepts the trade by pointer, as a caller may hand it.
		byPointer, err := codec.Marshal(&trade)
		if err != nil {
			t.Fatalf("%s: marshal by pointer: %v", name, err)
		}
		var again aggragates.Trades
		if err := codec.Unmarshal(byPointer, &again); err != nil {
			t.Fatalf("%s: unmarshal by pointer: %v", name, err)
		}
		assertEventsEqual(t, name+" by pointer", again.StrategyEvents, trade.StrategyEvents)
	}

	// The two hundred events are the case where the s2 pass earns its keep: the
	// documents repeat, so what the cache stores is far under the same trade as
	// JSON. A codec that stopped compressing would fail here.
	big := carrying(200)
	encoded, err := codec.Marshal(big)
	if err != nil {
		t.Fatal(err)
	}
	asJSON, err := json.Marshal(big)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded)*2 >= len(asJSON) {
		t.Fatalf("the cached form is %d bytes against %d as JSON: the s2 pass did not run", len(encoded), len(asJSON))
	}
}

// An event with no document at all — a nil Data, or the null a jsonb NULL
// column reads back as — is inert on every wire: it has no kind and decodes
// to the zero value, so no fold acts on it.
func TestAnEventWithoutADocumentStaysInertOnEveryWire(t *testing.T) {
	bare := aggragates.TradesStrategyEvents{ID: 1, TradeID: 7, Param: "cooldown", Gate: "firstFill", CreatedAt: eventsClock}
	trade := aggragates.Trades{ID: 7, StrategyEvents: []aggragates.TradesStrategyEvents{bare}}

	jsonForm, err := json.Marshal(trade)
	if err != nil {
		t.Fatal(err)
	}
	var fromJSON aggragates.Trades
	if err := json.Unmarshal(jsonForm, &fromJSON); err != nil {
		t.Fatalf("a null document must decode: %v", err)
	}

	codec := cache.New(&cache.Options{})
	cached, err := codec.Marshal(trade)
	if err != nil {
		t.Fatal(err)
	}
	var fromCache aggragates.Trades
	if err := codec.Unmarshal(cached, &fromCache); err != nil {
		t.Fatalf("a nil document must decode: %v", err)
	}

	for wire, decoded := range map[string]aggragates.Trades{"JSON": fromJSON, "the cache": fromCache} {
		if len(decoded.StrategyEvents) != 1 {
			t.Fatalf("%s: %d events, want the one", wire, len(decoded.StrategyEvents))
		}
		event := decoded.StrategyEvents[0]
		var data struct{ Event string }
		if event.Kind() != "" || event.DecodeData(&data) != nil || data.Event != "" {
			t.Errorf("%s: an event with no document must have no kind and decode to the zero value, got %q %+v", wire, event.Kind(), data)
		}
	}
}

// Whatever a list of events holds, both codecs hand it back: random lists over
// the params and gates, documents with the characters an encoder escapes,
// numbers of every shape, stamps present and absent.
func TestRandomEventListsSurviveBothCodecs(t *testing.T) {
	codec := cache.New(&cache.Options{})
	params := []string{"cooldown", "smartTakeLoss", "dynamicParams", "", "other"}
	kinds := []string{"activated", "armed", "entered", "held", "pending", "latched", "opened", ""}
	words := []string{"quiet", "leg > 60 bars", "a&b", "<b>", "naïve", " ", "日本", `"quoted"`, `back\slash`, "tab\there"}

	for seed := uint64(1); seed <= 40; seed++ {
		r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
		trade := aggragates.Trades{ID: uint(seed)}
		for i, count := 0, r.IntN(30); i < count; i++ {
			document := map[string]any{"event": kinds[r.IntN(len(kinds))]}
			if r.IntN(3) > 0 {
				document["price"] = math.Round(r.Float64()*2000000) / 100
			}
			if r.IntN(2) == 0 {
				reasons := make([]string, r.IntN(4))
				for j := range reasons {
					reasons[j] = words[r.IntN(len(words))]
				}
				document["reasons"] = reasons
			}
			var at time.Time
			if r.IntN(5) > 0 {
				at = eventsClock.Add(time.Duration(r.IntN(100_000_000)) * time.Millisecond)
			}
			event := aggragates.NewStrategyEvent(trade.ID, params[r.IntN(len(params))], params[r.IntN(len(params))], document, at)
			event.ID = uint(i + 1)
			trade.StrategyEvents = append(trade.StrategyEvents, event)
		}

		asJSON, err := json.Marshal(trade)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		var fromJSON aggragates.Trades
		if err := json.Unmarshal(asJSON, &fromJSON); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		assertEventsEqual(t, fmt.Sprintf("seed %d JSON", seed), fromJSON.StrategyEvents, trade.StrategyEvents)

		cached, err := codec.Marshal(trade)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		var fromCache aggragates.Trades
		if err := codec.Unmarshal(cached, &fromCache); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		assertEventsEqual(t, fmt.Sprintf("seed %d cache", seed), fromCache.StrategyEvents, trade.StrategyEvents)
	}
}

// What the constructor stores for a well-formed document is exactly its
// compact JSON, stamped and filed as asked, with no ID yet: the document reads
// back into the very struct that wrote it.
func TestNewStrategyEventStoresTheCompactDocument(t *testing.T) {
	at := clockAt(5)
	data := cooldown.FirstFillEvent{Event: cooldown.FirstFillArmed, Price: 97.4, Reference: 100, Anchor: 97.4, Inverse: true}

	event := aggragates.NewStrategyEvent(7, aggragates.StrategyParamCooldown, cooldown.GateFirstFill, data, at)

	want, _ := json.Marshal(data)
	if !bytes.Equal(event.Data, want) {
		t.Fatalf("Data = %s, want the compact document %s", event.Data, want)
	}
	if event.ID != 0 || event.TradeID != 7 || event.Param != "cooldown" || event.Gate != "firstFill" || !event.CreatedAt.Equal(at) {
		t.Fatalf("event = %+v, want trade 7, cooldown/firstFill, stamped %v, no ID", event, at)
	}
	if event.Kind() != "armed" {
		t.Fatalf("Kind = %q, want the document's event key", event.Kind())
	}
	var back cooldown.FirstFillEvent
	if err := event.DecodeData(&back); err != nil || back != data {
		t.Fatalf("DecodeData = %+v, %v, want %+v", back, err, data)
	}

	// An unstamped event stays unstamped: a fold reads that as an unknown clock.
	if unstamped := aggragates.NewStrategyEvent(7, "cooldown", "firstFill", data, time.Time{}); !unstamped.CreatedAt.IsZero() {
		t.Fatalf("a zero stamp must stay zero, got %v", unstamped.CreatedAt)
	}
}

// The constructor is total. What a document cannot carry — a NaN or an
// infinite float, a channel, a value that marshals to null or fails to
// marshal — must not lose the event or panic: the event is stored with an
// empty document, whose kind is unknown, so every fold skips it. Data is never
// nil, so the column is never written as NULL by a writer.
func TestNewStrategyEventIsTotal(t *testing.T) {
	type payload struct {
		Event string  `json:"event"`
		Price float64 `json:"price"`
	}
	var noPayload *payload
	at := clockAt(1)

	for name, data := range map[string]any{
		"a NaN inside the data":         payload{Event: "activated", Price: math.NaN()},
		"a positive infinity":           payload{Event: "activated", Price: math.Inf(1)},
		"a negative infinity":           payload{Event: "activated", Price: math.Inf(-1)},
		"nil data":                      nil,
		"a typed nil pointer":           noPayload,
		"a null raw message":            json.RawMessage("null"),
		"a raw message that is no JSON": json.RawMessage("{"),
		"a channel":                     make(chan int),
		"a function":                    func() {},
	} {
		event := aggragates.NewStrategyEvent(7, "cooldown", "firstFill", data, at)

		if event.Data == nil {
			t.Errorf("%s: Data is nil, a writer must never store a NULL document", name)
			continue
		}
		if string(event.Data) != "{}" {
			t.Errorf("%s: Data = %s, want the empty object {}: a null document would reach the column as SQL NULL", name, event.Data)
		}
		if event.Kind() != "" {
			t.Errorf("%s: Kind = %q, want none", name, event.Kind())
		}
		var decoded payload
		if err := event.DecodeData(&decoded); err != nil || decoded != (payload{}) {
			t.Errorf("%s: DecodeData = %+v, %v, want the zero value and no error", name, decoded, err)
		}
		if event.TradeID != 7 || event.Param != "cooldown" || event.Gate != "firstFill" || !event.CreatedAt.Equal(at) {
			t.Errorf("%s: the event must still be filed and stamped, got %+v", name, event)
		}
	}
}

// DecodeData leaves the caller's value as it was when the event has no
// document, and reports what it cannot read; Kind reads a document's event key
// and says nothing for one it cannot read.
func TestDecodeDataAndKindOnAwkwardDocuments(t *testing.T) {
	type payload struct {
		Event string `json:"event"`
	}
	empty := aggragates.TradesStrategyEvents{}
	kept := payload{Event: "kept"}
	if err := empty.DecodeData(&kept); err != nil || kept.Event != "kept" {
		t.Fatalf("an event with no document leaves the value alone, got %+v, %v", kept, err)
	}

	for name, tc := range map[string]struct {
		document string
		kind     string
		readable bool
	}{
		"a document":                 {`{"event":"held"}`, "held", true},
		"as Postgres renders it":     {`{"price": 1, "event": "held"}`, "held", true},
		"another key only":           {`{"price":1}`, "", true},
		"null":                       {`null`, "", true},
		"an array":                   {`[1]`, "", false},
		"text":                       {`"held"`, "", false},
		"an event that is no string": {`{"event":5}`, "", false},
		"a truncated document":       {`{"event":`, "", false},
	} {
		event := aggragates.TradesStrategyEvents{Data: json.RawMessage(tc.document)}
		if got := event.Kind(); got != tc.kind {
			t.Errorf("%s: Kind = %q, want %q", name, got, tc.kind)
		}
		var into payload
		if err := event.DecodeData(&into); (err == nil) != tc.readable {
			t.Errorf("%s: DecodeData error = %v, want readable %v", name, err, tc.readable)
		}
	}
}

// Every writer files its event under the stored (param, gate, kind), with a
// document that is a JSON object with a non-empty event key: the key Kind reads
// and every fold decodes. Compact, stamped like the row beside it, and within
// the width of the columns it lands in.
func TestEveryWriterFilesAKindedDocument(t *testing.T) {
	_, eventsSchema := parsedSchemas(t)
	paramWidth, gateWidth := columnWidth(t, eventsSchema, "Param"), columnWidth(t, eventsSchema, "Gate")
	trade := aggragates.Trades{ID: 7}
	at := clockAt(9)

	for _, tc := range everyWriter(trade, at) {
		event := tc.event
		if event.Param != tc.param || event.Gate != tc.gate {
			t.Errorf("%s: filed under %q/%q, want %q/%q", tc.name, event.Param, event.Gate, tc.param, tc.gate)
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(event.Data, &document); err != nil {
			t.Errorf("%s: Data %s is not a JSON object: %v", tc.name, event.Data, err)
			continue
		}
		var kind string
		if err := json.Unmarshal(document["event"], &kind); err != nil || kind == "" {
			t.Errorf("%s: Data %s has no non-empty event key (%v)", tc.name, event.Data, err)
		}
		if kind != tc.kind || event.Kind() != tc.kind {
			t.Errorf("%s: event key %q and Kind %q, want %q", tc.name, kind, event.Kind(), tc.kind)
		}
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, event.Data); err != nil || !bytes.Equal(compacted.Bytes(), event.Data) {
			t.Errorf("%s: Data %s is not compact", tc.name, event.Data)
		}
		if event.ID != 0 || event.TradeID != trade.ID || !event.CreatedAt.Equal(at) {
			t.Errorf("%s: event = %+v, want trade %d stamped %v and no ID", tc.name, event, trade.ID, at)
		}
		if len(event.Param) > paramWidth || len(event.Gate) > gateWidth {
			t.Errorf("%s: %q/%q does not fit the varchar(%d)/varchar(%d) columns", tc.name, event.Param, event.Gate, paramWidth, gateWidth)
		}
	}
}

// The two writers that hand back a pair, the smart take loss's Rows and the
// opened Rows, build the row and its event together: they carry one trade, one
// stamp and one price, so (TradeID, CreatedAt) finds the pair and the price the
// fold reads is the price the operator reads.
func TestAWritersRowAndItsEventArePairedByTradeStampAndPrice(t *testing.T) {
	trade := aggragates.Trades{ID: 7}
	at := clockAt(4)
	for name, row := range map[string]smarttakeloss.Row{
		"pending":   smarttakeloss.PendingRow("buy", 184.45, []string{"a"}),
		"cancelled": smarttakeloss.CancelledRow("buy", 179.78, nil),
		"reset":     smarttakeloss.ResetRow("stopLoss", 179.78),
		"latched":   smarttakeloss.LatchedRow("buy", 175.83, []string{"b", "c"}),
		"exit":      smarttakeloss.ExitRow(trade, "a rule that sold", 175.39),
	} {
		logRow, event := smarttakeloss.Rows(trade, row, at)
		if logRow.TradeID != event.TradeID || !logRow.CreatedAt.Equal(event.CreatedAt) {
			t.Errorf("%s: row (trade %d, %v) and event (trade %d, %v) are not a pair", name, logRow.TradeID, logRow.CreatedAt, event.TradeID, event.CreatedAt)
		}
		var data smarttakeloss.EventData
		if err := event.DecodeData(&data); err != nil || data.Price != logRow.Price || data.Price != row.Price {
			t.Errorf("%s: the event's price %v and the row's %v must be the one price, %v", name, data.Price, logRow.Price, err)
		}
	}

	row, event := dynamicparams.Opened{Points: 0.4, Depths: 1}.Rows(trade, 100, at)
	if row.TradeID != event.TradeID || !row.CreatedAt.Equal(event.CreatedAt) || row.Price != 100 {
		t.Errorf("opened: row %+v and event %+v are not a pair at the price named", row, event)
	}
}

// StrategyEventsOf keeps the order the trade carries the events in — the order
// every fold reads, which is the slice's and never the stamps' — and matches
// the param AND the gate exactly: not by prefix, not by case, not by the gate
// name alone. What it returns is its own slice.
func TestStrategyEventsOfKeepsSliceOrderAndFiltersByParamAndGate(t *testing.T) {
	event := func(id uint, param, gate string, minute int) aggragates.TradesStrategyEvents {
		return aggragates.TradesStrategyEvents{ID: id, Param: param, Gate: gate, CreatedAt: clockAt(minute), Data: json.RawMessage(`{}`)}
	}
	trade := aggragates.Trades{StrategyEvents: []aggragates.TradesStrategyEvents{
		event(1, "cooldown", "firstFill", 50),
		event(2, "cooldown", "depthSpacing", 40),
		event(3, "smartTakeLoss", "firstFill", 30),
		event(4, "cooldown", "firstFill", 10),
		event(5, "cooldownX", "firstFill", 60),
		event(6, "cooldown", "firstFillX", 60),
		event(7, "Cooldown", "firstFill", 60),
		event(8, "cooldown", "firstFill", 90),
		event(9, "", "", 5),
		event(10, "cooldown", "firstFill", 20),
	}}

	ids := func(matched []aggragates.TradesStrategyEvents) []uint {
		var out []uint
		for _, e := range matched {
			out = append(out, e.ID)
		}
		return out
	}

	got := trade.StrategyEventsOf("cooldown", "firstFill")
	if want := []uint{1, 4, 8, 10}; !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("StrategyEventsOf(cooldown, firstFill) = %v, want %v in slice order, whatever the stamps", ids(got), want)
	}
	if got := ids(trade.StrategyEventsOf("cooldown", "depthSpacing")); !reflect.DeepEqual(got, []uint{2}) {
		t.Fatalf("StrategyEventsOf(cooldown, depthSpacing) = %v, want [2]", got)
	}
	if got := ids(trade.StrategyEventsOf("smartTakeLoss", "firstFill")); !reflect.DeepEqual(got, []uint{3}) {
		t.Fatalf("the same gate name under another param is another gate, got %v", got)
	}
	for _, none := range [][2]string{{"cooldown", "opened"}, {"dynamicParams", "firstFill"}, {"cooldown", "FIRSTFILL"}} {
		if got := trade.StrategyEventsOf(none[0], none[1]); len(got) != 0 {
			t.Errorf("StrategyEventsOf(%q, %q) = %v, want none", none[0], none[1], ids(got))
		}
	}
	if got := (aggragates.Trades{}).StrategyEventsOf("cooldown", "firstFill"); len(got) != 0 {
		t.Fatalf("a trade with no events has none of any gate, got %d", len(got))
	}

	// The result is a new slice: writing to it never reaches the trade.
	got[0].Param = "overwritten"
	if trade.StrategyEvents[0].Param != "cooldown" {
		t.Fatal("StrategyEventsOf must not alias the trade's own slice")
	}
}

// AppendStrategyRow is the copy-on-append every writer that runs before the
// chain uses. The trade value a writer holds shares the backing arrays of the
// engine's own copy, so an append into spare capacity would write into an
// array another reader holds — and two writers appending from one shared copy
// would overwrite each other. Neither slice of the caller is ever written.
func TestAppendStrategyRowNeverWritesTheCallersArrays(t *testing.T) {
	logs := make([]aggragates.TradesLogs, 1, 8)
	logs[0] = aggragates.TradesLogs{ID: 1, Message: "first"}
	stored := make([]aggragates.TradesStrategyEvents, 1, 8)
	stored[0] = aggragates.TradesStrategyEvents{ID: 1, Param: "cooldown"}
	trade := aggragates.Trades{ID: 7, Symbol: "LINK/USDT", Logs: logs, StrategyEvents: stored}

	rowA := aggragates.TradesLogs{Message: "A", TradeID: 7}
	eventA := aggragates.NewStrategyEvent(7, "cooldown", "firstFill", map[string]string{"event": "activated"}, clockAt(1))
	rowB := aggragates.TradesLogs{Message: "B", TradeID: 7}
	eventB := aggragates.NewStrategyEvent(7, "smartTakeLoss", "slowDecline", map[string]string{"event": "pending"}, clockAt(2))

	first := aggragates.AppendStrategyRow(trade, rowA, eventA)
	second := aggragates.AppendStrategyRow(trade, rowB, eventB)

	for name, next := range map[string]struct {
		trade aggragates.Trades
		row   aggragates.TradesLogs
		event aggragates.TradesStrategyEvents
	}{"first": {first, rowA, eventA}, "second": {second, rowB, eventB}} {
		if len(next.trade.Logs) != 2 || len(next.trade.StrategyEvents) != 2 {
			t.Fatalf("%s: the pair joins the trade, got %d rows and %d events", name, len(next.trade.Logs), len(next.trade.StrategyEvents))
		}
		if next.trade.Logs[1].Message != next.row.Message || next.trade.StrategyEvents[1].Param != next.event.Param || !bytes.Equal(next.trade.StrategyEvents[1].Data, next.event.Data) {
			t.Errorf("%s: the pair is last, got row %q and event %q/%s", name, next.trade.Logs[1].Message, next.trade.StrategyEvents[1].Param, next.trade.StrategyEvents[1].Data)
		}
		if next.trade.Logs[0].Message != "first" || next.trade.StrategyEvents[0].ID != 1 {
			t.Errorf("%s: what the trade carried stays first", name)
		}
		if &next.trade.Logs[0] == &logs[0] || &next.trade.StrategyEvents[0] == &stored[0] {
			t.Errorf("%s: the result shares a backing array with the caller's slices", name)
		}
		if next.trade.ID != 7 || next.trade.Symbol != "LINK/USDT" {
			t.Errorf("%s: the rest of the trade must carry over", name)
		}
	}
	if first.Logs[1].Message != "A" || first.StrategyEvents[1].Param != "cooldown" {
		t.Fatal("a second append from the same trade overwrote the first result")
	}

	// The spare capacity of the caller's arrays is untouched, and so is the trade.
	if !reflect.DeepEqual(logs[:2][1], aggragates.TradesLogs{}) || !reflect.DeepEqual(stored[:2][1], aggragates.TradesStrategyEvents{}) {
		t.Fatal("the caller's spare capacity was written")
	}
	if len(trade.Logs) != 1 || len(trade.StrategyEvents) != 1 {
		t.Fatalf("the caller's trade must be unchanged, got %d rows and %d events", len(trade.Logs), len(trade.StrategyEvents))
	}

	// A trade with nothing yet takes its first pair.
	fresh := aggragates.AppendStrategyRow(aggragates.Trades{}, rowA, eventA)
	if len(fresh.Logs) != 1 || len(fresh.StrategyEvents) != 1 {
		t.Fatalf("an empty trade takes the pair, got %d rows and %d events", len(fresh.Logs), len(fresh.StrategyEvents))
	}
}

// parsedSchemas is GORM's own reading of the models, as AutoMigrate reads them:
// the trade with its relations, and the events' table.
func parsedSchemas(t *testing.T) (tradesSchema, eventsSchema *schema.Schema) {
	t.Helper()
	store := &sync.Map{}
	tradesSchema, err := schema.Parse(&aggragates.Trades{}, store, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("GORM cannot parse the trade: %v", err)
	}
	eventsSchema, err = schema.Parse(&aggragates.TradesStrategyEvents{}, store, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("GORM cannot parse the events: %v", err)
	}
	return tradesSchema, eventsSchema
}

// columnWidth is the n of a field's varchar(n) column type.
func columnWidth(t *testing.T, model *schema.Schema, field string) int {
	t.Helper()
	var width int
	if _, err := fmt.Sscanf(string(model.LookUpField(field).DataType), "varchar(%d)", &width); err != nil || width == 0 {
		t.Fatalf("%s must be a varchar(n) column, got %q", field, model.LookUpField(field).DataType)
	}
	return width
}

// What AutoMigrate builds from the model: a has-many from the trade to its
// events on TradeID with the cascade the deletes rely on, in the table
// trades_strategy_events, whose Data is a jsonb column written through the JSON
// serializer and never `not null` — the serializer writes a nil document as
// NULL — and whose trade key is indexed.
func TestTheEventsModelIsAHasManyOfTheTradeInItsOwnTable(t *testing.T) {
	tradesSchema, eventsSchema := parsedSchemas(t)

	relation, found := tradesSchema.Relationships.Relations["StrategyEvents"]
	if !found {
		t.Fatal("Trades has no StrategyEvents relation")
	}
	if relation.Type != schema.HasMany {
		t.Fatalf("StrategyEvents is a %q relation, want has-many", relation.Type)
	}
	if len(relation.References) != 1 {
		t.Fatalf("StrategyEvents joins on %d keys, want the one", len(relation.References))
	}
	reference := relation.References[0]
	if reference.PrimaryKey.Name != "ID" || reference.ForeignKey.Name != "TradeID" || reference.ForeignKey.DBName != "trade_id" || !reference.OwnPrimaryKey {
		t.Fatalf("StrategyEvents joins %s to %s (%s), want the trade's ID to the event's TradeID (trade_id)", reference.PrimaryKey.Name, reference.ForeignKey.Name, reference.ForeignKey.DBName)
	}
	if relation.FieldSchema.Table != "trades_strategy_events" || eventsSchema.Table != "trades_strategy_events" {
		t.Fatalf("the events' table is %q, want trades_strategy_events", eventsSchema.Table)
	}

	constraint := relation.ParseConstraint()
	if constraint == nil || constraint.OnDelete != "CASCADE" || constraint.OnUpdate != "CASCADE" || constraint.Name != "fk_trades_strategy_events" {
		t.Fatalf("the foreign key is %+v, want fk_trades_strategy_events with the cascade on delete and update", constraint)
	}

	if eventsSchema.PrioritizedPrimaryField == nil || eventsSchema.PrioritizedPrimaryField.Name != "ID" {
		t.Fatal("ID must be the primary key")
	}
	indexedTradeKey := false
	for _, index := range eventsSchema.ParseIndexes() {
		for _, option := range index.Fields {
			indexedTradeKey = indexedTradeKey || option.DBName == "trade_id"
		}
	}
	if !indexedTradeKey {
		t.Fatal("trade_id must be indexed: every load of a trade's events filters on it")
	}

	data := eventsSchema.LookUpField("Data")
	if data == nil || string(data.DataType) != "jsonb" {
		t.Fatalf("Data must be a jsonb column, got %+v", data)
	}
	if data.Serializer == nil {
		t.Fatal("Data must be written through a serializer")
	}
	if data.NotNull || data.TagSettings["NOT NULL"] != "" {
		t.Fatal("Data must never be `not null`: a nil document is written as NULL")
	}
	for _, name := range []string{"Param", "Gate"} {
		if got := string(eventsSchema.LookUpField(name).DataType); got != "varchar(32)" {
			t.Errorf("%s is a %q column, want varchar(32)", name, got)
		}
	}
}

// The insert path and the read path of the jsonb column, as GORM drives them
// with no database: the serializer's driver.Valuer hands the driver the
// document as text — NULL for a nil one — and Scan reads back any rendering of
// it, including Postgres' own (keys reordered, spaces after the separators).
func TestTheDataColumnRoundTripsThroughGORMsSerializer(t *testing.T) {
	_, eventsSchema := parsedSchemas(t)
	data := eventsSchema.LookUpField("Data")
	ctx := context.Background()

	valuer := func(event aggragates.TradesStrategyEvents) (driver.Value, error) {
		value, _ := data.ValueOf(ctx, reflect.ValueOf(&event).Elem())
		asDriver, ok := value.(driver.Valuer)
		if !ok {
			t.Fatalf("Data's value is a %T, want a driver.Valuer", value)
		}
		return asDriver.Value()
	}

	event := cooldown.NewFirstFillEvent(7, cooldown.FirstFillEvent{Event: cooldown.FirstFillArmed, Price: 97.4, Reference: 100, Anchor: 97.4}, clockAt(1))
	written, err := valuer(event)
	if err != nil || written != string(event.Data) {
		t.Fatalf("the column is written as %v (%v), want the document text %s", written, err, event.Data)
	}
	if written, err := valuer(aggragates.TradesStrategyEvents{}); err != nil || written != nil {
		t.Fatalf("a nil document is written as %v (%v), want SQL NULL", written, err)
	}
	// What a writer stores for data it cannot use is a document, never NULL: the
	// serializer turns a "null" document into SQL NULL, so the constructor's
	// fallback must be the empty object.
	for name, unusable := range map[string]any{"nil data": nil, "a NaN": math.NaN(), "a null raw message": json.RawMessage("null")} {
		written, err := valuer(aggragates.NewStrategyEvent(7, "cooldown", "firstFill", unusable, clockAt(1)))
		if err != nil || written != "{}" {
			t.Fatalf("%s: the column is written as %v (%v), want the empty object as text", name, written, err)
		}
	}

	rendered := `{"event": "armed", "price": 97.4, "anchor": 97.4, "reference": 100}`
	var read aggragates.TradesStrategyEvents
	if err := data.Serializer.Scan(ctx, data, reflect.ValueOf(&read).Elem(), []byte(rendered)); err != nil {
		t.Fatalf("Scan of the document Postgres renders: %v", err)
	}
	if string(read.Data) != rendered || read.Kind() != "armed" {
		t.Fatalf("read back %s (kind %q), want the rendering as it came", read.Data, read.Kind())
	}
	var armed cooldown.FirstFillEvent
	if err := read.DecodeData(&armed); err != nil || armed.Price != 97.4 || armed.Anchor != 97.4 || armed.Reference != 100 {
		t.Fatalf("the rendering must decode to the writer's values, got %+v, %v", armed, err)
	}

	// A NULL column reads back as an event with no document, which is inert.
	read = aggragates.TradesStrategyEvents{Data: json.RawMessage(`{"stale":true}`)}
	if err := data.Serializer.Scan(ctx, data, reflect.ValueOf(&read).Elem(), nil); err != nil {
		t.Fatalf("Scan of NULL: %v", err)
	}
	if len(read.Data) != 0 || read.Kind() != "" {
		t.Fatalf("a NULL column must read back with no document, got %s", read.Data)
	}
}
