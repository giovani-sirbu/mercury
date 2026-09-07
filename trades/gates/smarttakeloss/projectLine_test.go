package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

func TestProjectLineExtendsThroughTheAnchors(t *testing.T) {
	line := aggragates.TrendLine{
		From: aggragates.TrendLineAnchor{At: 1000, Price: 200},
		To:   aggragates.TrendLineAnchor{At: 2000, Price: 190},
	}
	cases := []struct {
		at   int64
		want float64
	}{
		{2000, 190}, // the newest anchor
		{3000, 180}, // one anchor gap ahead, still descending
		{1500, 195}, // between the anchors
	}
	for _, c := range cases {
		got, ok := projectLine(line, c.at)
		if !ok {
			t.Fatalf("at %d: expected a line", c.at)
		}
		testutil.AssertFloatEqual(t, got, c.want, 1e-9, "projection")
	}
}

func TestProjectLineNeedsTwoAnchorsAndAClock(t *testing.T) {
	if _, ok := projectLine(aggragates.TrendLine{}, 3000); ok {
		t.Fatal("the zero line is no line")
	}
	sameBar := aggragates.TrendLine{
		From: aggragates.TrendLineAnchor{At: 1000, Price: 200},
		To:   aggragates.TrendLineAnchor{At: 1000, Price: 190},
	}
	if _, ok := projectLine(sameBar, 3000); ok {
		t.Fatal("two anchors on the same bar are no line")
	}
	noPrice := aggragates.TrendLine{
		From: aggragates.TrendLineAnchor{At: 1000},
		To:   aggragates.TrendLineAnchor{At: 2000, Price: 190},
	}
	if _, ok := projectLine(noPrice, 3000); ok {
		t.Fatal("an anchor without a price is no line")
	}
	line := aggragates.TrendLine{
		From: aggragates.TrendLineAnchor{At: 1000, Price: 200},
		To:   aggragates.TrendLineAnchor{At: 2000, Price: 190},
	}
	if _, ok := projectLine(line, 0); ok {
		t.Fatal("an unknown clock projects nothing")
	}
}
