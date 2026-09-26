package fund

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The fund's tracker is an Excel workbook kept on OneDrive. The only thing we take from it
// is the trade log on the "Overall Fund" sheet (one row per buy or sell, sells have a negative
// quantity) plus the small options P&L table beside it. Everything else (capital, investors,
// NAV history, reports) lives in our own database.
//
// The Instrument column uses Excel's "Stocks" linked data type. Those cells read as #VALUE!
// to ordinary parsers, so we decode the rich value behind each cell ourselves: it carries the
// NSE ticker, the company name and Refinitiv's last price and previous close.

// Trade is one row of the trade log.
type Trade struct {
	Row      int       `json:"row"`
	TradeID  string    `json:"tradeId"`
	Date     time.Time `json:"date"`
	Pod      string    `json:"pod"`
	Symbol   string    `json:"symbol"`
	Name     string    `json:"name"`
	Qty      float64   `json:"qty"`
	Price    float64   `json:"price"`
	Stoploss string    `json:"stoploss"`
	Target   string    `json:"target"`
	Sector   string    `json:"sector"`
	Horizon  string    `json:"horizon"`
	Thesis   string    `json:"thesis"`
}

// PnLEntry is a realised P&L line that is not a stock trade (e.g. the Sensex 0DTE options book).
type PnLEntry struct {
	Row     int       `json:"row"`
	Book    string    `json:"book"`
	Date    time.Time `json:"date"`
	Gross   float64   `json:"gross"`
	Charges float64   `json:"charges"`
	Net     float64   `json:"net"`
}

// Quote is what Excel's linked data type knew about an instrument when the file was saved.
type Quote struct {
	Symbol    string    `json:"symbol"`
	Name      string    `json:"name"`
	Exchange  string    `json:"exchange"`
	Type      string    `json:"type"` // Stock | ETF | Index
	Industry  string    `json:"industry"`
	Price     float64   `json:"price"`
	PrevClose float64   `json:"prevClose"`
	At        time.Time `json:"at"` // last trade time
}

// Tracker is everything we read from one workbook.
type Tracker struct {
	Sheet  string           `json:"sheet"`
	Trades []Trade          `json:"trades"`
	Other  []PnLEntry       `json:"other"`
	Quotes map[string]Quote `json:"quotes"`
	// Sectors per symbol: Refinitiv's industry from the linked stock data; ETFs are classed
	// from their name (Gold, Liquid, International, Index).
	Sectors  map[string]string `json:"sectors"`
	Warnings []string          `json:"warnings"`
}

// Benchmark symbols as we store them. Excel's Stocks type calls them NIFTY and NIFTY500.
const (
	Nifty50  = "NIFTY50"
	Nifty500 = "NIFTY500"
)

var indexAliases = map[string]string{"NIFTY": Nifty50, "NIFTY 50": Nifty50, "NIFTY500": Nifty500, "NIFTY 500": Nifty500}

// ParseTracker reads the trade log from an .xlsx file. sheetName may be empty: then the first
// sheet with a "Trade ID" header row is used.
func ParseTracker(data []byte, sheetName string) (*Tracker, error) {
	wb, err := openWorkbook(data)
	if err != nil {
		return nil, err
	}
	names := wb.sheetNames
	if sheetName != "" {
		names = []string{sheetName}
		if _, ok := wb.sheetPaths[sheetName]; !ok {
			return nil, fmt.Errorf("the workbook has no sheet called %q (sheets: %s)", sheetName, strings.Join(wb.sheetNames, ", "))
		}
	}
	for _, name := range names {
		sh, err := wb.sheet(name)
		if err != nil {
			return nil, err
		}
		if hr, cols := findTradeHeader(sh); hr > 0 {
			t := wb.readTracker(name, sh, hr, cols)
			wb.fillSectors(t)
			return t, nil
		}
	}
	return nil, fmt.Errorf("no sheet has a trade log (a header row with Trade ID, Entry Date, Pod, Instrument, Quantity and Entry Price)")
}

// ---------- the trade log ----------

func normHeader(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var headerAliases = map[string][]string{
	"id":       {"tradeid", "id", "tradeno"},
	"date":     {"entrydate", "tradedate", "date"},
	"pod":      {"pod"},
	"inst":     {"instrument", "stock", "symbol", "ticker"},
	"qty":      {"quantity", "qty"},
	"price":    {"entryprice", "price", "tradeprice"},
	"stoploss": {"stoploss", "sl"},
	"target":   {"target"},
	"sector":   {"industry", "sector"},
	"horizon":  {"horizon"},
	"thesis":   {"thesis"},
	"name":     {"instrumentnamehelper", "instrumentname", "companyname", "name"},
}

var requiredCols = []string{"date", "pod", "inst", "qty", "price"}

func findTradeHeader(sh *sheet) (int, map[string]int) {
	for _, r := range sh.rowNums() {
		if r > 200 {
			break
		}
		cols := map[string]int{}
		for c, cell := range sh.rows[r] {
			h := normHeader(cell.text())
			for key, aliases := range headerAliases {
				for _, a := range aliases {
					if h == a {
						if _, taken := cols[key]; !taken {
							cols[key] = c
						}
					}
				}
			}
		}
		ok := true
		for _, k := range requiredCols {
			if _, found := cols[k]; !found {
				ok = false
			}
		}
		if ok {
			return r, cols
		}
	}
	return 0, nil
}

var tickerInText = regexp.MustCompile(`\((?:XNSE|XBOM|NSE|BSE):([A-Z0-9&_.\-]+)\)`)

func (wb *workbook) readTracker(name string, sh *sheet, hr int, cols map[string]int) *Tracker {
	t := &Tracker{Sheet: name, Quotes: map[string]Quote{}}
	warn := func(row int, f string, a ...any) {
		t.Warnings = append(t.Warnings, fmt.Sprintf("Row %d: ", row)+fmt.Sprintf(f, a...))
	}
	get := func(r int, key string) *cell {
		c, ok := cols[key]
		if !ok {
			return nil
		}
		return sh.rows[r][c]
	}
	text := func(r int, key string) string {
		c := get(r, key)
		if c == nil || c.isError() {
			return ""
		}
		return strings.TrimSpace(c.text())
	}

	// Every linked-data cell on the sheet (including the index comparison block) gives us a quote.
	for _, r := range sh.rowNums() {
		for _, c := range sh.rows[r] {
			if q, ok := wb.quote(c); ok {
				// A stock linked in several rows can hold snapshots from different refreshes; keep the latest.
				if old, seen := t.Quotes[q.Symbol]; !seen || q.At.After(old.At) {
					t.Quotes[q.Symbol] = q
				}
			}
		}
	}

	for _, r := range sh.rowNums() {
		if r <= hr {
			continue
		}
		dc, qc, pc, ic := get(r, "date"), get(r, "qty"), get(r, "price"), get(r, "inst")
		if dc.empty() && qc.empty() && ic.empty() {
			continue
		}
		tr := Trade{Row: r, TradeID: text(r, "id"), Pod: text(r, "pod"), Sector: text(r, "sector"),
			Horizon: text(r, "horizon"), Thesis: text(r, "thesis"), Stoploss: text(r, "stoploss"), Target: text(r, "target")}
		if tr.TradeID == "" {
			tr.TradeID = "R" + strconv.Itoa(r)
		}
		var ok bool
		if tr.Date, ok = dc.date(); !ok {
			warn(r, "the entry date %q is not a date, so the trade was skipped", dc.text())
			continue
		}
		if tr.Qty, ok = qc.number(); !ok || tr.Qty == 0 {
			warn(r, "the quantity %q is not a number, so the trade was skipped", qc.text())
			continue
		}
		if tr.Price, ok = pc.number(); !ok || tr.Price <= 0 {
			warn(r, "the entry price %q is not a positive number, so the trade was skipped", pc.text())
			continue
		}
		if q, isQuote := wb.quote(ic); isQuote {
			tr.Symbol, tr.Name = q.Symbol, q.Name
		} else if s := strings.TrimSpace(ic.text()); s != "" && !ic.isError() {
			if m := tickerInText.FindStringSubmatchIndex(s); m != nil {
				tr.Symbol, tr.Name = s[m[2]:m[3]], strings.TrimSpace(s[:m[0]])
			} else {
				tr.Symbol, tr.Name = strings.ToUpper(strings.Fields(s)[0]), s
			}
		}
		if tr.Symbol == "" {
			warn(r, "the instrument could not be read (it shows #VALUE! and has no linked stock data), so the trade was skipped")
			continue
		}
		if n := text(r, "name"); tr.Name == "" && n != "" {
			tr.Name = n
		}
		if tr.Pod == "" {
			tr.Pod = "Unassigned"
		}
		t.Trades = append(t.Trades, tr)
	}
	t.Other = readOtherPnL(sh, hr, warn)
	return t
}

// fillSectors picks one sector per symbol (see Tracker.Sectors).
func (wb *workbook) fillSectors(t *Tracker) {
	t.Sectors = map[string]string{}
	for _, tr := range t.Trades {
		if _, ok := t.Sectors[tr.Symbol]; ok {
			continue
		}
		q, ok := t.Quotes[tr.Symbol]
		switch {
		case ok && q.Industry != "":
			t.Sectors[tr.Symbol] = q.Industry
		case ok && strings.EqualFold(q.Type, "ETF"):
			t.Sectors[tr.Symbol] = ETFClass(q.Name)
		}
	}
}

// ETFClass groups an ETF by what it holds, from its name.
func ETFClass(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "gold") || strings.Contains(n, "silver"):
		return "Gold & Silver ETFs"
	case strings.Contains(n, "liq") || strings.Contains(n, "1d rate") || strings.Contains(n, "overnight") || strings.Contains(n, "money market"):
		return "Liquid ETFs"
	case strings.Contains(n, "nasdaq") || strings.Contains(n, "s&p 500") || strings.Contains(n, "hang seng") || strings.Contains(n, "us "):
		return "International ETFs"
	}
	return "Index ETFs"
}

// readOtherPnL finds small realised-P&L tables above the trade log, such as
//
//	Sensex 0dte
//	Trade Date | Gross P&L | Brokerage & Levies | Net P&L
//	24-09-2026 | 323       | 354.52             | -31.52
//	Total      | ...
func readOtherPnL(sh *sheet, tradeHeader int, warn func(int, string, ...any)) []PnLEntry {
	var out []PnLEntry
	for _, r := range sh.rowNums() {
		if r >= tradeHeader {
			break
		}
		dateCol, grossCol, chargesCol, netCol := -1, -1, -1, -1
		for c, cell := range sh.rows[r] {
			switch h := normHeader(cell.text()); {
			case h == "tradedate" || h == "date":
				dateCol = c
			case strings.HasPrefix(h, "grosspl") || h == "grosspnl":
				grossCol = c
			case strings.HasPrefix(h, "brokerage") || h == "charges":
				chargesCol = c
			case strings.HasPrefix(h, "netpl") || h == "netpnl":
				netCol = c
			}
		}
		if dateCol < 0 || (netCol < 0 && grossCol < 0) {
			continue
		}
		book := "Other"
		if above := sh.rows[r-1][dateCol]; above != nil && strings.TrimSpace(above.text()) != "" {
			book = strings.TrimSpace(above.text())
		}
		for rr := r + 1; rr < tradeHeader; rr++ {
			dc := sh.rows[rr][dateCol]
			d, ok := dc.date()
			if !ok {
				break // "Total" row or the end of the table
			}
			e := PnLEntry{Row: rr, Book: book, Date: d}
			e.Gross, _ = sh.rows[rr][grossCol].number()
			e.Charges, _ = sh.rows[rr][chargesCol].number()
			if n, ok := sh.rows[rr][netCol].number(); ok {
				e.Net = n
			} else {
				e.Net = e.Gross - e.Charges
			}
			out = append(out, e)
		}
	}
	return out
}

// ---------- minimal xlsx reader ----------

type cell struct {
	typ string // s (shared string), str, inlineStr, b, e (error), n or ""
	val string
	vm  int // value-metadata index (1-based) for rich values, 0 if none
}

func (c *cell) empty() bool { return c == nil || (c.val == "" && c.vm == 0) }
func (c *cell) isError() bool {
	return c != nil && c.typ == "e"
}
func (c *cell) text() string {
	if c == nil {
		return ""
	}
	return c.val
}
func (c *cell) number() (float64, bool) {
	if c == nil || c.isError() || c.val == "" {
		return 0, false
	}
	s := strings.ReplaceAll(strings.TrimSpace(c.val), ",", "")
	s = strings.TrimPrefix(s, "₹")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

var dateLayouts = []string{"2006-01-02", "02-01-2006", "02/01/2006", "2 Jan 2006", "02 Jan 2006", "2-Jan-2006", "02-Jan-2006", "Jan 2, 2006", "2006/01/02"}

func (c *cell) date() (time.Time, bool) {
	if c == nil || c.isError() {
		return time.Time{}, false
	}
	if c.typ == "" || c.typ == "n" {
		if f, ok := c.number(); ok && f > 20000 && f < 80000 {
			return excelDate(f), true
		}
		return time.Time{}, false
	}
	s := strings.TrimSpace(c.val)
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// excelDate turns an Excel serial (days since 1899-12-30) into a date at midnight UTC.
func excelDate(f float64) time.Time {
	d := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(math.Floor(f)))
	return d
}

func excelTime(f float64) time.Time {
	return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).Add(time.Duration(f * 24 * float64(time.Hour)))
}

type sheet struct {
	rows map[int]map[int]*cell
}

func (s *sheet) rowNums() []int {
	out := make([]int, 0, len(s.rows))
	for r := range s.rows {
		out = append(out, r)
	}
	sort.Ints(out)
	return out
}

type workbook struct {
	files      map[string]*zip.File
	sheetNames []string
	sheetPaths map[string]string
	strings    []string
	rich       *richData
}

func openWorkbook(data []byte) (*workbook, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("this is not an .xlsx file (%v)", err)
	}
	wb := &workbook{files: map[string]*zip.File{}, sheetPaths: map[string]string{}}
	for _, f := range zr.File {
		wb.files[strings.TrimPrefix(f.Name, "/")] = f
	}

	var book struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := wb.decode("xl/workbook.xml", &book); err != nil {
		return nil, fmt.Errorf("this is not an .xlsx file (no workbook): %v", err)
	}
	var rels struct {
		Rel []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := wb.decode("xl/_rels/workbook.xml.rels", &rels); err != nil {
		return nil, err
	}
	targets := map[string]string{}
	for _, r := range rels.Rel {
		t := r.Target
		if strings.HasPrefix(t, "/") {
			t = strings.TrimPrefix(t, "/")
		} else {
			t = path.Join("xl", t)
		}
		targets[r.ID] = t
	}
	for _, s := range book.Sheets {
		wb.sheetNames = append(wb.sheetNames, s.Name)
		wb.sheetPaths[s.Name] = targets[s.RID]
	}

	if _, ok := wb.files["xl/sharedStrings.xml"]; ok {
		var sst struct {
			SI []struct {
				T string `xml:"t"`
				R []struct {
					T string `xml:"t"`
				} `xml:"r"`
			} `xml:"si"`
		}
		if err := wb.decode("xl/sharedStrings.xml", &sst); err != nil {
			return nil, err
		}
		for _, si := range sst.SI {
			s := si.T
			for _, r := range si.R {
				s += r.T
			}
			wb.strings = append(wb.strings, s)
		}
	}
	wb.rich = loadRichData(wb)
	return wb, nil
}

func (wb *workbook) open(name string) (io.ReadCloser, error) {
	f, ok := wb.files[name]
	if !ok {
		return nil, fmt.Errorf("missing %s", name)
	}
	return f.Open()
}

func (wb *workbook) decode(name string, v any) error {
	rc, err := wb.open(name)
	if err != nil {
		return err
	}
	defer rc.Close()
	return xml.NewDecoder(rc).Decode(v)
}

var cellRef = regexp.MustCompile(`^([A-Z]+)(\d+)$`)

func colIndex(letters string) int {
	n := 0
	for _, r := range letters {
		n = n*26 + int(r-'A'+1)
	}
	return n - 1
}

func (wb *workbook) sheet(name string) (*sheet, error) {
	rc, err := wb.open(wb.sheetPaths[name])
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	sh := &sheet{rows: map[int]map[int]*cell{}}
	dec := xml.NewDecoder(rc)
	var cur *cell
	var inV, inT bool
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("sheet %q: %v", name, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "c":
				cur = &cell{}
				var ref string
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "r":
						ref = a.Value
					case "t":
						cur.typ = a.Value
					case "vm":
						cur.vm, _ = strconv.Atoi(a.Value)
					}
				}
				if m := cellRef.FindStringSubmatch(ref); m != nil {
					r, _ := strconv.Atoi(m[2])
					if sh.rows[r] == nil {
						sh.rows[r] = map[int]*cell{}
					}
					sh.rows[r][colIndex(m[1])] = cur
				}
			case "v":
				inV = cur != nil
			case "t":
				inT = cur != nil && cur.typ == "inlineStr"
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "c":
				if cur != nil && cur.typ == "s" {
					if i, err := strconv.Atoi(cur.val); err == nil && i >= 0 && i < len(wb.strings) {
						cur.val = wb.strings[i]
					}
				}
				cur = nil
			case "v":
				inV = false
			case "t":
				inT = false
			}
		case xml.CharData:
			if inV || inT {
				cur.val += string(t)
			}
		}
	}
	return sh, nil
}

// ---------- rich values (Excel "Stocks" linked data type) ----------

type richData struct {
	valueMeta [][2]int     // vm (1-based) -> (metadata type index 1-based, future-metadata index)
	typeNames []string     // metadata types, 1-based via index-1
	richIndex []int        // XLRICHVALUE future metadata -> rich value index
	values    []richValue  // rdrichvalue.xml
	structs   []richStruct // rdrichvaluestructure.xml
}

type richValue struct {
	s    int
	vals []string
}

type richStruct struct {
	t    string
	keys []string
}

func loadRichData(wb *workbook) *richData {
	if _, ok := wb.files["xl/metadata.xml"]; !ok {
		return nil
	}
	var md struct {
		Types []struct {
			Name string `xml:"name,attr"`
		} `xml:"metadataTypes>metadataType"`
		Future []struct {
			Name string `xml:"name,attr"`
			BK   []struct {
				RVB []struct {
					I int `xml:"i,attr"`
				} `xml:"extLst>ext>rvb"`
			} `xml:"bk"`
		} `xml:"futureMetadata"`
		Value []struct {
			RC []struct {
				T int `xml:"t,attr"`
				V int `xml:"v,attr"`
			} `xml:"rc"`
		} `xml:"valueMetadata>bk"`
	}
	if wb.decode("xl/metadata.xml", &md) != nil {
		return nil
	}
	rd := &richData{}
	for _, t := range md.Types {
		rd.typeNames = append(rd.typeNames, t.Name)
	}
	for _, f := range md.Future {
		if f.Name != "XLRICHVALUE" {
			continue
		}
		for _, bk := range f.BK {
			i := -1
			if len(bk.RVB) > 0 {
				i = bk.RVB[0].I
			}
			rd.richIndex = append(rd.richIndex, i)
		}
	}
	for _, bk := range md.Value {
		if len(bk.RC) > 0 {
			rd.valueMeta = append(rd.valueMeta, [2]int{bk.RC[0].T, bk.RC[0].V})
		} else {
			rd.valueMeta = append(rd.valueMeta, [2]int{0, 0})
		}
	}
	var st struct {
		S []struct {
			T string `xml:"t,attr"`
			K []struct {
				N string `xml:"n,attr"`
			} `xml:"k"`
		} `xml:"s"`
	}
	if wb.decode("xl/richData/rdrichvaluestructure.xml", &st) != nil {
		return nil
	}
	for _, s := range st.S {
		rs := richStruct{t: s.T}
		for _, k := range s.K {
			rs.keys = append(rs.keys, k.N)
		}
		rd.structs = append(rd.structs, rs)
	}
	var rv struct {
		RV []struct {
			S int      `xml:"s,attr"`
			V []string `xml:"v"`
		} `xml:"rv"`
	}
	if wb.decode("xl/richData/rdrichvalue.xml", &rv) != nil {
		return nil
	}
	for _, v := range rv.RV {
		rd.values = append(rd.values, richValue{s: v.S, vals: v.V})
	}
	return rd
}

// entity returns the key/value pairs of the linked entity behind a cell, following
// _linkedentity -> _linkedentitycore references.
func (rd *richData) entity(vm int) map[string]string {
	if rd == nil || vm < 1 || vm > len(rd.valueMeta) {
		return nil
	}
	m := rd.valueMeta[vm-1]
	if m[0] < 1 || m[0] > len(rd.typeNames) || rd.typeNames[m[0]-1] != "XLRICHVALUE" || m[1] < 0 || m[1] >= len(rd.richIndex) {
		return nil
	}
	idx := rd.richIndex[m[1]]
	for depth := 0; depth < 4; depth++ {
		if idx < 0 || idx >= len(rd.values) {
			return nil
		}
		v := rd.values[idx]
		if v.s < 0 || v.s >= len(rd.structs) {
			return nil
		}
		s := rd.structs[v.s]
		kv := map[string]string{"_type": s.t}
		for i, k := range s.keys {
			if i < len(v.vals) {
				kv[k] = v.vals[i]
			}
		}
		if s.t == "_linkedentity" {
			next, err := strconv.Atoi(kv["%cvi"])
			if err != nil {
				return nil
			}
			idx = next
			continue
		}
		return kv
	}
	return nil
}

func (wb *workbook) quote(c *cell) (Quote, bool) {
	if c == nil || c.vm == 0 {
		return Quote{}, false
	}
	kv := wb.rich.entity(c.vm)
	sym := strings.TrimSpace(kv["Ticker symbol"])
	if kv == nil || sym == "" {
		return Quote{}, false
	}
	q := Quote{Symbol: sym, Name: kv["Name"], Exchange: kv["Exchange abbreviation"], Type: kv["Instrument type"], Industry: kv["Industry"]}
	if q.Type == "Index" {
		if a, ok := indexAliases[strings.ToUpper(sym)]; ok {
			q.Symbol = a
		} else {
			q.Symbol = "IDX:" + strings.ToUpper(sym)
		}
	}
	q.Price, _ = strconv.ParseFloat(kv["Price"], 64)
	q.PrevClose, _ = strconv.ParseFloat(kv["Previous close"], 64)
	if f, err := strconv.ParseFloat(kv["Last trade time"], 64); err == nil {
		q.At = excelTime(f)
	}
	return q, true
}
