package fund

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// Service owns the sync pipeline and the computed snapshot the API serves.
type Service struct {
	app core.App

	ExcelURL    string // sharing link to the tracker; empty: uploads only
	TradesSheet string // "" = find the sheet with a Trade ID header
	Yahoo       *Yahoo // nil disables price history downloads
	HTTP        *http.Client

	syncMu    sync.Mutex // one sync at a time
	computeMu sync.Mutex // one recompute at a time
	mu        sync.RWMutex
	snap      *Snapshot
}

// Snapshot is the fund, computed after each sync.
type Snapshot struct {
	Result   *Result
	Risk     Risk
	Monthly  []MonthRow
	Computed time.Time
	Settings Settings
	Sectors  []Sector
	LastSync time.Time
}

type Settings struct {
	StartNAV        float64 `json:"startNav"`
	RiskFree        float64 `json:"riskFree"`
	ManagerNote     string  `json:"managerNote"`
	ManagerNoteBy   string  `json:"managerNoteBy"`
	ManagerNoteDate string  `json:"managerNoteDate"`
	Motto           string  `json:"motto"`
}

type Sector struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Weight float64 `json:"weight"`
}

func New(app core.App) *Service {
	return &Service{app: app, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// Snap returns the latest snapshot (computing it on first use).
func (s *Service) Snap() *Snapshot {
	s.mu.RLock()
	sn := s.snap
	s.mu.RUnlock()
	// Refresh now and then too, so imports from the CLI (another process) show up.
	if sn != nil && time.Since(sn.Computed) < 10*time.Minute {
		return sn
	}
	s.computeMu.Lock()
	s.mu.RLock()
	fresh := s.snap != nil && time.Since(s.snap.Computed) < 10*time.Minute
	s.mu.RUnlock()
	if !fresh {
		if err := s.Recompute(); err != nil {
			s.app.Logger().Error("recompute failed", slog.String("error", err.Error()))
		}
	}
	s.computeMu.Unlock()
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snap == nil {
		return &Snapshot{Result: &Result{}}
	}
	return s.snap
}

// Invalidate drops the snapshot; the next request recomputes it.
func (s *Service) Invalidate() {
	s.mu.Lock()
	s.snap = nil
	s.mu.Unlock()
}

// ---------- sync ----------

// SyncSummary is what a sync reports back to the admin page.
type SyncSummary struct {
	OK       bool     `json:"ok"`
	Message  string   `json:"message"`
	Trades   int      `json:"trades"`
	Warnings []string `json:"warnings"`
}

// ErrNeedsSignIn means the sharing link returned a sign-in page instead of the file.
var ErrNeedsSignIn = errors.New("the tracker link asks for a Microsoft sign-in, so the server can't download it. In OneDrive, share the file as \"Anyone with the link can view\", or upload the .xlsx on this page instead")

// DownloadURL turns a OneDrive/SharePoint sharing link into a direct download.
func DownloadURL(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return link
	}
	h := strings.ToLower(u.Host)
	if strings.HasSuffix(h, "sharepoint.com") || strings.HasSuffix(h, "1drv.ms") || strings.HasSuffix(h, "onedrive.live.com") {
		q := u.Query()
		q.Set("download", "1")
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// FetchExcel downloads the tracker from its sharing link.
func (s *Service) FetchExcel(ctx context.Context) ([]byte, error) {
	if s.ExcelURL == "" {
		return nil, errors.New("no tracker link is set (EXCEL_URL in .env). Upload the .xlsx instead")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DownloadURL(s.ExcelURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "AuxesisCapital/1.0")
	res, err := s.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("couldn't reach the tracker link: %v", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return nil, ErrNeedsSignIn
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("the tracker link answered HTTP %d", res.StatusCode)
	}
	if !bytes.HasPrefix(data, []byte("PK")) {
		return nil, ErrNeedsSignIn
	}
	return data, nil
}

// SyncFromLink downloads and imports the tracker.
func (s *Service) SyncFromLink(ctx context.Context, by string) (*SyncSummary, error) {
	data, err := s.FetchExcel(ctx)
	if err != nil {
		s.logSync("link", false, err.Error(), 0, nil, by)
		return nil, err
	}
	return s.Import(data, "link", by)
}

// Import replaces the trade log with the one in the workbook, records the prices it carries
// and recomputes the fund.
func (s *Service) Import(data []byte, source, by string) (*SyncSummary, error) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	tr, err := ParseTracker(data, s.TradesSheet)
	if err != nil {
		s.logSync(source, false, err.Error(), 0, nil, by)
		return nil, err
	}
	if len(tr.Trades) == 0 {
		err := errors.New("the tracker has no trades we could read, so nothing was changed")
		s.logSync(source, false, err.Error(), 0, tr.Warnings, by)
		return nil, err
	}
	err = s.app.RunInTransaction(func(tx core.App) error {
		if _, err := tx.DB().NewQuery("DELETE FROM trades").Execute(); err != nil {
			return err
		}
		if _, err := tx.DB().NewQuery("DELETE FROM other_pnl").Execute(); err != nil {
			return err
		}
		tc, err := tx.FindCollectionByNameOrId("trades")
		if err != nil {
			return err
		}
		names := map[string]string{}
		for _, t := range tr.Trades {
			r := core.NewRecord(tc)
			r.Load(map[string]any{
				"tradeId": t.TradeID, "row": t.Row, "date": dateDay(t.Date), "pod": t.Pod, "symbol": t.Symbol, "name": t.Name,
				"qty": t.Qty, "price": t.Price, "stoploss": t.Stoploss, "target": t.Target, "sector": t.Sector,
				"horizon": t.Horizon, "thesis": t.Thesis,
			})
			if err := tx.SaveNoValidate(r); err != nil {
				return err
			}
			names[t.Symbol] = t.Name
		}
		oc, err := tx.FindCollectionByNameOrId("other_pnl")
		if err != nil {
			return err
		}
		for _, o := range tr.Other {
			r := core.NewRecord(oc)
			r.Load(map[string]any{"book": o.Book, "date": dateDay(o.Date), "gross": o.Gross, "charges": o.Charges, "net": o.Net, "row": o.Row})
			if err := tx.SaveNoValidate(r); err != nil {
				return err
			}
		}

		// Instruments: everything traded plus the benchmarks.
		ic, err := tx.FindCollectionByNameOrId("instruments")
		if err != nil {
			return err
		}
		syms := map[string]bool{Nifty50: true, Nifty500: true}
		for sym := range names {
			syms[sym] = true
		}
		for sym := range syms {
			q, hasQ := tr.Quotes[sym]
			r, _ := tx.FindFirstRecordByData("instruments", "symbol", sym)
			if r == nil {
				r = core.NewRecord(ic)
				r.Set("symbol", sym)
			}
			if n := names[sym]; n != "" {
				r.Set("name", n)
			}
			if hasQ {
				if q.Name != "" {
					r.Set("name", q.Name)
				}
				r.Set("exchange", q.Exchange)
				r.Set("type", q.Type)
				r.Set("industry", q.Industry)
				if q.Price > 0 {
					r.Set("lastPrice", q.Price)
					r.Set("prevClose", q.PrevClose)
					r.Set("priceAt", q.At.Format(time.RFC3339))
				}
			} else if sym == Nifty50 {
				r.Set("name", "Nifty 50")
				r.Set("type", "Index")
			} else if sym == Nifty500 {
				r.Set("name", "Nifty 500")
				r.Set("type", "Index")
			}
			if sec := tr.Sectors[sym]; sec != "" && !r.GetBool("sectorLocked") {
				r.Set("sector", sec)
			}
			if err := tx.SaveNoValidate(r); err != nil {
				return err
			}
		}

		// Prices the workbook carries: the last price (on its trade day) and the previous close.
		for _, q := range tr.Quotes {
			if q.Price <= 0 || q.At.IsZero() {
				continue
			}
			d := DayOf(q.At)
			if err := upsertPrice(tx, q.Symbol, d, q.Price, "excel"); err != nil {
				return err
			}
			if q.PrevClose > 0 {
				if pd := prevWeekday(d); pd != "" {
					if err := insertPriceIfMissing(tx, q.Symbol, pd, q.PrevClose); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		s.logSync(source, false, "Import failed: "+err.Error(), 0, tr.Warnings, by)
		return nil, err
	}
	s.Invalidate()
	sum := &SyncSummary{OK: true, Trades: len(tr.Trades), Warnings: tr.Warnings}
	sum.Message = fmt.Sprintf("Imported %d trades from %q", len(tr.Trades), tr.Sheet)
	if len(tr.Other) > 0 {
		sum.Message += fmt.Sprintf(" and %d options P&L line(s)", len(tr.Other))
	}
	sum.Message += "."
	if err := s.Recompute(); err != nil {
		sum.Warnings = append(sum.Warnings, "NAV could not be recomputed: "+err.Error())
	} else if w := s.Snap().Result.Warnings; len(w) > 0 {
		sum.Warnings = append(sum.Warnings, w...)
	}
	s.logSync(source, true, sum.Message, len(tr.Trades), sum.Warnings, by)
	return sum, nil
}

func prevWeekday(d Day) Day {
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return ""
	}
	for {
		t = t.AddDate(0, 0, -1)
		if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
			return t.Format("2006-01-02")
		}
	}
}

var sourceRank = map[string]int{"excel": 1, "yahoo": 2, "manual": 3}

func upsertPrice(tx core.App, sym string, d Day, v float64, source string) error {
	r, _ := tx.FindFirstRecordByFilter("prices", "symbol = {:s} && day = {:d}", map[string]any{"s": sym, "d": d})
	if r != nil {
		if sourceRank[r.GetString("source")] > sourceRank[source] {
			return nil
		}
		if r.GetFloat("close") == v && r.GetString("source") == source {
			return nil
		}
	} else {
		c, err := tx.FindCollectionByNameOrId("prices")
		if err != nil {
			return err
		}
		r = core.NewRecord(c)
		r.Set("symbol", sym)
		r.Set("day", d)
	}
	r.Set("close", v)
	r.Set("source", source)
	return tx.SaveNoValidate(r)
}

func insertPriceIfMissing(tx core.App, sym string, d Day, v float64) error {
	r, _ := tx.FindFirstRecordByFilter("prices", "symbol = {:s} && day = {:d}", map[string]any{"s": sym, "d": d})
	if r != nil {
		return nil
	}
	return upsertPrice(tx, sym, d, v, "excel")
}

func (s *Service) logSync(source string, ok bool, msg string, trades int, warnings []string, by string) {
	c, err := s.app.FindCollectionByNameOrId("sync_log")
	if err != nil {
		return
	}
	if warnings == nil {
		warnings = []string{}
	}
	r := core.NewRecord(c)
	r.Load(map[string]any{"source": source, "ok": ok, "message": msg, "trades": trades, "warnings": warnings, "by": by})
	if err := s.app.Save(r); err != nil {
		s.app.Logger().Error("sync log", slog.String("error", err.Error()))
	}
	if ok {
		s.app.Logger().Info("tracker sync", slog.String("source", source), slog.String("message", msg))
	} else {
		s.app.Logger().Warn("tracker sync failed", slog.String("source", source), slog.String("message", msg))
	}
}

// ---------- price history ----------

// BackfillPrices downloads daily closes from inception (or the last stored day) for every
// instrument ever traded and both benchmarks. full re-downloads everything.
func (s *Service) BackfillPrices(ctx context.Context, full bool) (int, []string) {
	if s.Yahoo == nil {
		return 0, []string{"Price history downloads are off (PRICE_HISTORY=off)."}
	}
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	start := s.inceptionDay()
	if start == "" {
		return 0, []string{"There are no trades or capital flows yet."}
	}
	from, _ := time.ParseInLocation("2006-01-02", start, IST)
	to := time.Now().In(IST)

	insts, err := s.app.FindAllRecords("instruments")
	if err != nil {
		return 0, []string{err.Error()}
	}
	lastDay := map[string]Day{}
	if !full {
		var rows []struct {
			Symbol string `db:"symbol"`
			Day    string `db:"day"`
		}
		_ = s.app.DB().NewQuery("SELECT symbol, MAX(day) AS day FROM prices WHERE source = 'yahoo' GROUP BY symbol").All(&rows)
		for _, r := range rows {
			lastDay[r.Symbol] = r.Day
		}
	}
	var errs []string
	n := 0
	for _, inst := range insts {
		sym := inst.GetString("symbol")
		f := from
		if d, ok := lastDay[sym]; ok && d > start {
			if t, err := time.ParseInLocation("2006-01-02", d, IST); err == nil {
				f = t.AddDate(0, 0, -5) // refetch a few days: late corrections, today's final close
			}
		}
		closes, err := s.Yahoo.Closes(ctx, YahooSymbol(sym, inst.GetString("yahoo")), f, to)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		err = s.app.RunInTransaction(func(tx core.App) error {
			for d, v := range closes {
				if err := upsertPrice(tx, sym, d, v, "yahoo"); err != nil {
					return err
				}
				n++
			}
			return nil
		})
		if err != nil {
			errs = append(errs, sym+": "+err.Error())
		}
		select {
		case <-ctx.Done():
			return n, append(errs, ctx.Err().Error())
		case <-time.After(250 * time.Millisecond): // be polite to Yahoo
		}
	}
	s.Invalidate()
	msg := fmt.Sprintf("Downloaded %d daily closes for %d instruments.", n, len(insts))
	if len(errs) > 0 {
		msg += fmt.Sprintf(" %d could not be fetched.", len(errs))
	}
	s.logSync("prices", len(errs) < len(insts), msg, 0, errs, "")
	return n, errs
}

func (s *Service) inceptionDay() Day {
	var d struct {
		D string `db:"d"`
	}
	_ = s.app.DB().NewQuery("SELECT MIN(d) AS d FROM (SELECT MIN(date) AS d FROM trades UNION ALL SELECT MIN(date) FROM capital_flows) WHERE d IS NOT NULL AND d != ''").One(&d)
	return d.D
}

// ---------- compute ----------

// Recompute rebuilds the snapshot from the database and stores the NAV series.
func (s *Service) Recompute() error {
	settings := s.loadSettings()
	trades, other, flows, prices, sectors, err := s.loadInputs()
	if err != nil {
		return err
	}
	asOf := today()
	// Value as of the latest day we have any price or trade, never in the future.
	latest := ""
	for _, m := range prices {
		for d := range m {
			if d > latest {
				latest = d
			}
		}
	}
	for _, t := range trades {
		if d := dateDay(t.Date); d > latest {
			latest = d
		}
	}
	if latest != "" && latest < asOf {
		asOf = latest
	}
	res := Compute(trades, other, flows, prices, settings.StartNAV, asOf, sectors)
	sn := &Snapshot{Result: res, Computed: time.Now(), Settings: settings}
	sn.Risk = Analyse(res.Series, res.StartNAV, settings.RiskFree/100)
	sn.Monthly = Monthly(res.Series, res.StartNAV)

	agg := map[string]float64{}
	for _, p := range res.Positions {
		agg[p.Sector] += p.Value
	}
	var total float64
	if n := len(res.Series); n > 0 {
		total = res.Series[n-1].Value
		agg["Cash"] = res.Series[n-1].Cash
	}
	for name, v := range agg {
		w := 0.0
		if total > 0 {
			w = v / total
		}
		sn.Sectors = append(sn.Sectors, Sector{Name: name, Value: v, Weight: w})
	}
	sort.Slice(sn.Sectors, func(i, j int) bool {
		if (sn.Sectors[i].Name == "Cash") != (sn.Sectors[j].Name == "Cash") {
			return sn.Sectors[j].Name == "Cash"
		}
		return sn.Sectors[i].Value > sn.Sectors[j].Value
	})
	if rows, err := s.app.FindRecordsByFilter("sync_log", "ok = true && source != 'prices'", "-created", 1, 0); err == nil && len(rows) > 0 {
		sn.LastSync = rows[0].GetDateTime("created").Time()
	}

	s.mu.Lock()
	s.snap = sn
	s.mu.Unlock()
	return s.storeNav(res.Series)
}

func today() Day { return time.Now().In(IST).Format("2006-01-02") }

func (s *Service) storeNav(series []NavPoint) error {
	return s.app.RunInTransaction(func(tx core.App) error {
		c, err := tx.FindCollectionByNameOrId("nav")
		if err != nil {
			return err
		}
		existing, err := tx.FindAllRecords("nav")
		if err != nil {
			return err
		}
		byDay := map[string]*core.Record{}
		for _, r := range existing {
			byDay[r.GetString("day")] = r
		}
		keep := map[string]bool{}
		for _, p := range series {
			keep[p.Date] = true
			r := byDay[p.Date]
			if r == nil {
				r = core.NewRecord(c)
				r.Set("day", p.Date)
			} else if r.GetFloat("nav") == p.NAV && r.GetFloat("value") == p.Value && r.GetFloat("nifty50") == p.Nifty50 && r.GetFloat("nifty500") == p.Nifty500 {
				continue
			}
			r.Load(map[string]any{"nav": p.NAV, "cash": p.Cash, "holdings": p.Holdings, "value": p.Value, "units": p.Units,
				"nifty50": p.Nifty50, "nifty500": p.Nifty500, "positions": p.Positions})
			if err := tx.SaveNoValidate(r); err != nil {
				return err
			}
		}
		for d, r := range byDay {
			if !keep[d] {
				if err := tx.Delete(r); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *Service) loadSettings() Settings {
	st := Settings{StartNAV: 1000, RiskFree: 6.5, Motto: "Per ardua ad alta"}
	rows, err := s.app.FindRecordsByFilter("fund_settings", "", "created", 1, 0)
	if err != nil || len(rows) == 0 {
		return st
	}
	r := rows[0]
	if v := r.GetFloat("startNav"); v > 0 {
		st.StartNAV = v
	}
	if v := r.GetFloat("riskFree"); v > 0 {
		st.RiskFree = v
	}
	st.ManagerNote, st.ManagerNoteBy, st.ManagerNoteDate = r.GetString("managerNote"), r.GetString("managerNoteBy"), r.GetString("managerNoteDate")
	if m := r.GetString("motto"); m != "" {
		st.Motto = m
	}
	return st
}

func (s *Service) loadInputs() ([]Trade, []PnLEntry, []Flow, Prices, map[string]string, error) {
	var trades []Trade
	tr, err := s.app.FindAllRecords("trades")
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	for _, r := range tr {
		d, _ := time.Parse("2006-01-02", r.GetString("date"))
		trades = append(trades, Trade{Row: r.GetInt("row"), TradeID: r.GetString("tradeId"), Date: d, Pod: r.GetString("pod"),
			Symbol: r.GetString("symbol"), Name: r.GetString("name"), Qty: r.GetFloat("qty"), Price: r.GetFloat("price"),
			Sector: r.GetString("sector"), Horizon: r.GetString("horizon"), Thesis: r.GetString("thesis")})
	}
	var other []PnLEntry
	or, err := s.app.FindAllRecords("other_pnl")
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	for _, r := range or {
		d, _ := time.Parse("2006-01-02", r.GetString("date"))
		other = append(other, PnLEntry{Row: r.GetInt("row"), Book: r.GetString("book"), Date: d, Gross: r.GetFloat("gross"), Charges: r.GetFloat("charges"), Net: r.GetFloat("net")})
	}
	var flows []Flow
	fr, err := s.app.FindAllRecords("capital_flows")
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	for _, r := range fr {
		flows = append(flows, Flow{Date: r.GetString("date"), Amount: r.GetFloat("amount"), Units: r.GetFloat("units"), Note: r.GetString("note")})
	}
	prices := Prices{}
	var rows []struct {
		Symbol string  `db:"symbol"`
		Day    string  `db:"day"`
		Close  float64 `db:"close"`
	}
	if err := s.app.DB().NewQuery("SELECT symbol, day, close FROM prices").All(&rows); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	for _, r := range rows {
		prices.Set(r.Symbol, r.Day, r.Close)
	}
	sectors := map[string]string{}
	ins, err := s.app.FindAllRecords("instruments")
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	for _, r := range ins {
		if sec := r.GetString("sector"); sec != "" {
			sectors[r.GetString("symbol")] = sec
		}
	}
	return trades, other, flows, prices, sectors, nil
}
