package fund

import (
	"archive/zip"
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

// buildTracker writes a small workbook shaped like the fund's tracker: an options P&L table
// above a trade log whose Instrument cells are Excel "Stocks" linked data types.
func buildTracker(t *testing.T) []byte {
	t.Helper()
	type stock struct {
		ticker, name, industry string
		price, prev            float64
	}
	stocks := []stock{{"HDFCBANK", "HDFC BANK LIMITED", "Banking Services", 1000, 990}, {"TITAN", "TITAN COMPANY LIMITED", "Specialty Retailers", 3000, 3030}}
	// Rich values: a _linkedentitycore per stock, each wrapped by a _linkedentity pointing at it.
	var rv strings.Builder
	for i, s := range stocks {
		fmt.Fprintf(&rv, `<rv s="0"><v>en-US</v><v>%s</v><v>%s</v><v>%s</v><v>XNSE</v><v>Stock</v><v>%g</v><v>%g</v><v>46290.43</v></rv>`, s.ticker, s.name, s.industry, s.price, s.prev)
		fmt.Fprintf(&rv, `<rv s="1"><v>%d</v></rv>`, 2*i)
	}
	structs := `<rvStructures xmlns="http://schemas.microsoft.com/office/spreadsheetml/2017/richdata" count="2">` +
		`<s t="_linkedentitycore"><k n="%EntityCulture" t="s"/><k n="Ticker symbol" t="s"/><k n="Name" t="s"/><k n="Industry" t="s"/><k n="Exchange abbreviation" t="s"/><k n="Instrument type" t="s"/><k n="Price"/><k n="Previous close"/><k n="Last trade time"/></s>` +
		`<s t="_linkedentity"><k n="%cvi" t="r"/></s></rvStructures>`
	// vm 1 -> future bk 0 -> rich value 1 (HDFCBANK wrapper); vm 2 -> bk 1 -> rich value 3 (TITAN)
	metadata := `<metadata xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:xlrd="http://schemas.microsoft.com/office/spreadsheetml/2017/richdata">` +
		`<metadataTypes count="2"><metadataType name="XLDAPR"/><metadataType name="XLRICHVALUE"/></metadataTypes>` +
		`<futureMetadata name="XLRICHVALUE" count="2"><bk><extLst><ext uri="x"><xlrd:rvb i="1"/></ext></extLst></bk><bk><extLst><ext uri="x"><xlrd:rvb i="3"/></ext></extLst></bk></futureMetadata>` +
		`<valueMetadata count="2"><bk><rc t="2" v="0"/></bk><bk><rc t="2" v="1"/></bk></valueMetadata></metadata>`

	str := func(ref, s string) string {
		return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, s)
	}
	num := func(ref string, v float64) string { return fmt.Sprintf(`<c r="%s"><v>%g</v></c>`, ref, v) }
	stockCell := func(ref string, vm int) string {
		return fmt.Sprintf(`<c r="%s" t="e" vm="%d"><v>#VALUE!</v></c>`, ref, vm)
	}
	rows := []string{
		`<row r="1">` + str("E1", "Sensex 0dte") + `</row>`,
		`<row r="2">` + str("E2", "Trade Date") + str("F2", "Gross P&amp;L") + str("G2", "Brokerage &amp; Levies") + str("H2", "Net P&amp;L") + `</row>`,
		`<row r="3">` + num("E3", 46289) + num("F3", 500) + num("G3", 100) + num("H3", 400) + `</row>`,
		`<row r="4">` + str("E4", "Total") + `</row>`,
		`<row r="6">` + str("A6", "Trade ID") + str("B6", "Entry Date") + str("C6", "Pod") + str("D6", "Instrument") + str("E6", "Quantity") + str("F6", "Entry Price") + str("N6", "Industry") + `</row>`,
		// 13 Jul 2026 = 46216
		`<row r="7">` + num("A7", 1) + num("B7", 46216) + str("C7", "PodA") + stockCell("D7", 1) + num("E7", 10) + num("F7", 900) + str("N7", "Banks") + `</row>`,
		`<row r="8">` + num("A8", 2) + num("B8", 46216) + str("C8", "PodB") + stockCell("D8", 2) + num("E8", 5) + num("F8", 3200) + `</row>`,
		`<row r="9">` + num("A9", 3) + num("B9", 46218) + str("C9", "PodA") + stockCell("D9", 1) + num("E9", -4) + num("F9", 950) + `</row>`,
		`<row r="10">` + num("A10", 4) + num("B10", 46218) + str("C10", "PodB") + str("D10", "INFO EDGE (INDIA) LIMITED (XNSE:NAUKRI)") + num("E10", 2) + num("F10", 1200) + `</row>`,
		`<row r="11">` + num("A11", 5) + num("B11", 46218) + str("C11", "PodB") + `<c r="D11" t="e"><v>#VALUE!</v></c>` + num("E11", 1) + num("F11", 10) + `</row>`,
	}
	sheet := `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` + strings.Join(rows, "") + `</sheetData></worksheet>`
	files := map[string]string{
		"xl/workbook.xml":                      `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Notes" sheetId="2" r:id="rId2"/><sheet name="Overall Fund" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels":           `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Target="/xl/worksheets/sheet2.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":             sheet,
		"xl/worksheets/sheet2.xml":             `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1">` + str("A1", "hello") + `</row></sheetData></worksheet>`,
		"xl/metadata.xml":                      metadata,
		"xl/richData/rdrichvaluestructure.xml": structs,
		"xl/richData/rdrichvalue.xml":          `<rvData xmlns="http://schemas.microsoft.com/office/spreadsheetml/2017/richdata">` + rv.String() + `</rvData>`,
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	return buf.Bytes()
}

func TestParseTracker(t *testing.T) {
	tr, err := ParseTracker(buildTracker(t), "")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Sheet != "Overall Fund" {
		t.Errorf("sheet %q", tr.Sheet)
	}
	if len(tr.Trades) != 4 {
		t.Fatalf("want 4 trades (one unreadable row skipped), got %d: %+v", len(tr.Trades), tr.Trades)
	}
	if len(tr.Warnings) != 1 || !strings.Contains(tr.Warnings[0], "Row 11") {
		t.Errorf("warnings %v", tr.Warnings)
	}
	a := tr.Trades[0]
	if a.Symbol != "HDFCBANK" || a.Name != "HDFC BANK LIMITED" || a.Pod != "PodA" || a.Qty != 10 || a.Price != 900 || a.Date.Format("2006-01-02") != "2026-07-13" {
		t.Errorf("trade 1: %+v", a)
	}
	if tr.Trades[2].Qty != -4 {
		t.Errorf("sell qty %v", tr.Trades[2].Qty)
	}
	if n := tr.Trades[3]; n.Symbol != "NAUKRI" || n.Name != "INFO EDGE (INDIA) LIMITED" {
		t.Errorf("text instrument: %+v", n)
	}
	// Sector: Refinitiv's industry, not the tracker's Industry column.
	if tr.Sectors["HDFCBANK"] != "Banking Services" || tr.Sectors["TITAN"] != "Specialty Retailers" {
		t.Errorf("sectors %v", tr.Sectors)
	}
	q := tr.Quotes["TITAN"]
	if q.Price != 3000 || q.PrevClose != 3030 || DayOf(q.At) != "2026-09-25" {
		t.Errorf("quote %+v (%s)", q, DayOf(q.At))
	}
	if len(tr.Other) != 1 || tr.Other[0].Net != 400 || tr.Other[0].Book != "Sensex 0dte" {
		t.Errorf("other %+v", tr.Other)
	}
}

func TestParseTrackerErrors(t *testing.T) {
	if _, err := ParseTracker([]byte("<html>sign in</html>"), ""); err == nil {
		t.Error("html accepted")
	}
	if _, err := ParseTracker(buildTracker(t), "Missing"); err == nil || !strings.Contains(err.Error(), "Overall Fund") {
		t.Errorf("unknown sheet: %v", err)
	}
	if _, err := ParseTracker(buildTracker(t), "Notes"); err == nil {
		t.Error("sheet without trades accepted")
	}
}

func day(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }

func TestCompute(t *testing.T) {
	trades := []Trade{
		{Row: 1, Date: day("2026-07-13"), Pod: "A", Symbol: "X", Qty: 10, Price: 100},
		{Row: 2, Date: day("2026-07-14"), Pod: "A", Symbol: "X", Qty: -4, Price: 120}, // realise +80
		{Row: 3, Date: day("2026-07-14"), Pod: "B", Symbol: "Y", Qty: 5, Price: 50},
	}
	prices := Prices{}
	prices.Set("X", "2026-07-13", 110)
	prices.Set("X", "2026-07-15", 130)
	prices.Set(Nifty50, "2026-07-13", 1000)
	prices.Set(Nifty50, "2026-07-14", 1010)
	prices.Set(Nifty50, "2026-07-15", 1020)
	flows := []Flow{{Date: "2026-07-13", Amount: 10000, Units: 10}, {Date: "2026-07-15", Amount: 1000}}
	other := []PnLEntry{{Date: day("2026-07-14"), Net: -20}}
	res := Compute(trades, other, flows, prices, 1000, "2026-07-15", map[string]string{"Y": "Autos"})

	if len(res.Series) != 3 {
		t.Fatalf("series %+v", res.Series)
	}
	d1 := res.Series[0] // cash 10000-1000 = 9000, X 10 @110
	if d1.Cash != 9000 || d1.Holdings != 1100 || math.Abs(d1.NAV-1010) > 1e-9 {
		t.Errorf("day 1 %+v", d1)
	}
	d2 := res.Series[1] // cash 9000+480-250-20 = 9210; X 6 @110 (last close), Y 5 @50 (trade price, no market price)
	if d2.Cash != 9210 || d2.Holdings != 660+250 {
		t.Errorf("day 2 %+v", d2)
	}
	nav2 := (9210.0 + 910) / 10
	d3 := res.Series[2] // new money buys units at day 2's NAV
	if math.Abs(d3.Units-(10+1000/nav2)) > 1e-9 {
		t.Errorf("units %v", d3.Units)
	}
	if res.Realised != 80 || res.Other != -20 {
		t.Errorf("realised %v other %v", res.Realised, res.Other)
	}
	if len(res.Positions) != 2 {
		t.Fatalf("positions %+v", res.Positions)
	}
	x := res.Positions[0]
	if x.Symbol != "X" || x.Qty != 6 || x.AvgCost != 100 || x.Price != 130 || x.PrevClose != 110 || math.Abs(x.Return-0.3) > 1e-9 || x.Stale {
		t.Errorf("X %+v", x)
	}
	y := res.Positions[1]
	if !y.Stale || y.Sector != "Autos" || len(res.Warnings) != 1 {
		t.Errorf("Y %+v warnings %v", y, res.Warnings)
	}
	if res.Pods[0].Pod != "A" || res.Pods[0].Realised != 80 || res.Pods[1].Open != 1 {
		t.Errorf("pods %+v", res.Pods)
	}
}

// A stock sold out and bought again: the return is measured from the average of all its buys,
// not just the latest one.
func TestReturnFromAverageBuy(t *testing.T) {
	trades := []Trade{
		{Row: 1, Date: day("2026-07-13"), Symbol: "Z", Qty: 10, Price: 100},
		{Row: 2, Date: day("2026-07-14"), Symbol: "Z", Qty: -10, Price: 110},
		{Row: 3, Date: day("2026-07-15"), Symbol: "Z", Qty: 10, Price: 140},
		{Row: 4, Date: day("2026-07-15"), Symbol: "Z", Qty: 20, Price: 120},
	}
	prices := Prices{}
	prices.Set("Z", "2026-07-15", 150)
	res := Compute(trades, nil, []Flow{{Date: "2026-07-13", Amount: 10000, Units: 10}}, prices, 1000, "2026-07-15", nil)
	z := res.Positions[0]
	// Buys: 10@100, 10@140, 20@120 → 4800/40 = 120.
	if z.AvgBuy != 120 || math.Abs(z.Return-0.25) > 1e-9 {
		t.Errorf("avg buy %v return %v", z.AvgBuy, z.Return)
	}
	if math.Abs(z.AvgCost-380.0/3) > 1e-9 { // accounting cost only counts the shares bought after the exit
		t.Errorf("avg cost %v", z.AvgCost)
	}
}

func TestBookShortAndFlip(t *testing.T) {
	var b book
	b.apply(10, 100, "d1")
	if r := b.apply(-15, 120, "d2"); r != 200 || b.qty != -5 || b.cost != -600 {
		t.Errorf("flip: realised %v qty %v cost %v", r, b.qty, b.cost)
	}
	if r := b.apply(5, 100, "d3"); r != 100 || b.qty != 0 {
		t.Errorf("cover: realised %v qty %v", r, b.qty)
	}
}

func TestAnalyse(t *testing.T) {
	var s []NavPoint
	nav, idx := 1000.0, 20000.0
	d := day("2025-01-01")
	for i := 0; i < 400; i++ {
		m := 0.01 * math.Sin(float64(i))
		idx *= 1 + m
		nav *= 1 + 0.5*m + 0.0004
		s = append(s, NavPoint{Date: d.AddDate(0, 0, i).Format("2006-01-02"), NAV: nav, Nifty50: idx})
	}
	r := Analyse(s, 1000, 0.065, BenchNifty50)
	if math.Abs(r.Beta-0.5) > 0.01 {
		t.Errorf("beta %v", r.Beta)
	}
	if !r.Annualised || r.Alpha <= 0 || r.MaxDrawdown >= 0 || r.UpCapture <= 0 {
		t.Errorf("risk %+v", r)
	}
	rows := Monthly(s, 1000)
	if len(rows) != 2 || rows[0].Months[0] == nil || rows[1].Months[11] != nil {
		t.Errorf("monthly %+v", rows)
	}
}

// The Nifty 500 is the main benchmark; each benchmark's statistics use its own levels.
func TestAnalyseBenchmarks(t *testing.T) {
	var s []NavPoint
	d := day("2026-07-13")
	for i := 0; i < 60; i++ {
		s = append(s, NavPoint{Date: d.AddDate(0, 0, i).Format("2006-01-02"), NAV: 1000 + float64(i),
			Nifty50: 20000 * (1 + 0.001*float64(i)), Nifty500: 18000 * (1 - 0.001*float64(i))})
	}
	r500 := Analyse(s, 1000, 0.065, BenchNifty500)
	r50 := Analyse(s, 1000, 0.065, BenchNifty50)
	if r500.Benchmark != "Nifty 500" || r50.Benchmark != "Nifty 50" {
		t.Errorf("names %q %q", r500.Benchmark, r50.Benchmark)
	}
	if math.Abs(r500.BenchReturn-(-0.059)) > 1e-9 || math.Abs(r50.BenchReturn-0.059) > 1e-9 {
		t.Errorf("returns 500 %v, 50 %v", r500.BenchReturn, r50.BenchReturn)
	}
	for i := range s {
		s[i].Nifty500 = 0
	}
	if Analyse(s, 1000, 0.065, BenchNifty500).HasBenchmark {
		t.Error("no Nifty 500 prices should mean no Nifty 500 statistics")
	}
}

func TestXIRR(t *testing.T) {
	r, ok := XIRR([]CashFlow{{day("2025-01-01"), -1000}, {day("2026-01-01"), 1100}})
	if !ok || math.Abs(r-0.1) > 1e-4 {
		t.Errorf("xirr %v %v", r, ok)
	}
}

func TestDownloadURL(t *testing.T) {
	got := DownloadURL("https://iima1-my.sharepoint.com/:x:/g/personal/x/ABC?e=1")
	if got != "https://iima1-my.sharepoint.com/personal/x/_layouts/15/download.aspx?share=ABC" {
		t.Error(got)
	}
	if DownloadURL("https://example.com/f.xlsx") != "https://example.com/f.xlsx" {
		t.Error("non-OneDrive link changed")
	}
}

func TestETFClass(t *testing.T) {
	for name, want := range map[string]string{"Nippon India ETF Gold BeES": "Gold & Silver ETFs", "Zerodha Nifty 1D Rate Liq ETF": "Liquid ETFs",
		"Motilal Oswal NASDAQ 100 ETF": "International ETFs", "Nippon IN ETF Nifty Bank BeES": "Index ETFs"} {
		if got := ETFClass(name); got != want {
			t.Errorf("%s: %s", name, got)
		}
	}
}

func TestPreviewOf(t *testing.T) {
	body := "Lead.\n\nSecond.\n\n## Heading\n\nHidden."
	if got := previewOf(body, 3); got != "Lead.\n\nSecond." {
		t.Errorf("%q", got)
	}
}
