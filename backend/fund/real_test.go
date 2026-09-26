package fund

import (
	"math"
	"os"
	"testing"
)

// Runs against a real tracker when AUXESIS_TRACKER points at one (it holds fund data, so it is not committed).
func TestRealTracker(t *testing.T) {
	p := os.Getenv("AUXESIS_TRACKER")
	if p == "" {
		t.Skip("set AUXESIS_TRACKER=/path/to/tracker.xlsx")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	tr, err := ParseTracker(data, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("sheet %q: %d trades, %d other, %d quotes, %d warnings", tr.Sheet, len(tr.Trades), len(tr.Other), len(tr.Quotes), len(tr.Warnings))
	for _, w := range tr.Warnings {
		t.Log("warn:", w)
	}
	prices := Prices{}
	for _, q := range tr.Quotes {
		prices.Set(q.Symbol, DayOf(q.At), q.Price)
	}
	flows := []Flow{{Date: "2026-07-13", Amount: 1238342.2, Units: 1238.3422}}
	res := Compute(tr.Trades, nil, flows, prices, 1000, "2026-09-25", nil)
	last := res.Series[len(res.Series)-1]
	t.Logf("NAV %.4f value %.2f cash %.2f holdings %.2f positions %d", last.NAV, last.Value, last.Cash, last.Holdings, last.Positions)
	for _, w := range res.Warnings {
		t.Log("calc:", w)
	}
	for _, p := range res.Pods {
		t.Logf("pod %+v", p)
	}
	if math.Abs(last.NAV-1018.0154) > 0.5 {
		t.Errorf("NAV %.4f, the sheet says 1018.0154", last.NAV)
	}
	t.Logf("other: %+v", tr.Other)
}
