package fund

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// ---------- public ----------

type GrowthPoint struct {
	D string   `json:"d"`
	F float64  `json:"f"`           // fund: value of ₹1,000 invested when the cycle began (= NAV per unit)
	B *float64 `json:"b,omitempty"` // Nifty 500 (main benchmark): ₹1,000 invested at the close of the cycle's first day
	C *float64 `json:"c,omitempty"` // Nifty 50, the same way
}

type PublicView struct {
	HasData    bool          `json:"hasData"`
	AsOf       string        `json:"asOf"`
	Inception  string        `json:"inception"`
	FundITD    float64       `json:"fundItd"`
	Nifty50ITD *float64      `json:"nifty50Itd"`
	Nifty500   *float64      `json:"nifty500Itd"`
	Positions  int           `json:"positions"`
	NAV        float64       `json:"nav"`
	Growth     []GrowthPoint `json:"growth"`
	Motto      string        `json:"motto"`
}

func (s *Service) Public() PublicView {
	sn := s.Snap()
	res := sn.Result
	v := PublicView{Motto: sn.Settings.Motto, Growth: []GrowthPoint{}}
	if len(res.Series) < 1 || res.Series[len(res.Series)-1].Units == 0 {
		return v
	}
	v.HasData, v.AsOf, v.Inception = true, res.AsOf, res.Inception
	v.Growth = growth(res.Series, res.StartNAV, 260)
	last := res.Series[len(res.Series)-1]
	v.FundITD = last.NAV/res.StartNAV - 1
	v.NAV = round(last.NAV, 2)
	if r := sn.Risk50; r.HasBenchmark {
		b := r.BenchReturn
		v.Nifty50ITD = &b
	}
	if r := sn.Risk; r.HasBenchmark && r.Benchmark == BenchNifty500.Name {
		b := r.BenchReturn
		v.Nifty500 = &b
	}
	v.Positions = len(res.Positions)
	return v
}

// growth is the value of ₹1,000 put into the fund (at the ₹1,000 issue price) and into the
// Nifty 500 and Nifty 50 when the cycle began, keeping at most max points.
func growth(series []NavPoint, startNAV float64, max int) []GrowthPoint {
	if len(series) == 0 {
		return nil
	}
	step := int(math.Ceil(float64(len(series)) / float64(max)))
	if step < 1 {
		step = 1
	}
	f0 := startNAV
	if f0 <= 0 {
		f0 = series[0].NAV
	}
	b0 := firstPos(series, func(p NavPoint) float64 { return p.Nifty500 })
	c0 := firstPos(series, func(p NavPoint) float64 { return p.Nifty50 })
	var out []GrowthPoint
	for i, p := range series {
		if i%step != 0 && i != len(series)-1 {
			continue
		}
		g := GrowthPoint{D: p.Date, F: round(p.NAV/f0*1000, 2)}
		if c0 > 0 && p.Nifty50 > 0 {
			c := round(p.Nifty50/c0*1000, 2)
			g.C = &c
		}
		if b0 > 0 && p.Nifty500 > 0 {
			b := round(p.Nifty500/b0*1000, 2)
			g.B = &b
		}
		out = append(out, g)
	}
	return out
}

func round(x float64, d int) float64 {
	p := math.Pow(10, float64(d))
	return math.Round(x*p) / p
}

// ---------- investors ----------

type Txn struct {
	ID     string  `json:"id"`
	Date   string  `json:"date"`
	Kind   string  `json:"kind"`
	Amount float64 `json:"amount"`
	Units  float64 `json:"units"`
	NAV    float64 `json:"nav"`
	Note   string  `json:"note"`
}

type Holding struct {
	InvestorID string   `json:"id"`
	Name       string   `json:"name"`
	Email      string   `json:"email"`
	Folio      string   `json:"folio"`
	Programme  string   `json:"programme"`
	Units      float64  `json:"units"`
	Invested   float64  `json:"invested"`
	Value      float64  `json:"value"`
	Return     float64  `json:"return"`
	XIRR       *float64 `json:"xirr"`
	Since      string   `json:"since"`
	Txns       []Txn    `json:"txns"`
}

// navOn is the fund's NAV at the close before d (units bought on d are priced at it).
func navBefore(series []NavPoint, d string, start float64) float64 {
	i := sort.Search(len(series), func(i int) bool { return series[i].Date >= d })
	if i == 0 {
		return start
	}
	return series[i-1].NAV
}

func (s *Service) holding(inv *core.Record, sn *Snapshot) (*Holding, error) {
	rows, err := s.app.FindRecordsByFilter("investor_txns", "investor = {:id}", "date", 0, 0, map[string]any{"id": inv.Id})
	if err != nil {
		return nil, err
	}
	h := &Holding{InvestorID: inv.Id, Name: inv.GetString("name"), Email: inv.GetString("email"), Folio: inv.GetString("folio"), Programme: inv.GetString("programme"), Txns: []Txn{}}
	series := sn.Result.Series
	var cfs []CashFlow
	for _, r := range rows {
		t := Txn{ID: r.Id, Date: r.GetString("date"), Kind: r.GetString("kind"), Amount: r.GetFloat("amount"), Units: r.GetFloat("units"), Note: r.GetString("note")}
		if t.NAV = r.GetFloat("nav"); t.NAV <= 0 {
			t.NAV = navBefore(series, t.Date, sn.Result.StartNAV)
		}
		if t.Units == 0 && t.Amount != 0 && t.NAV > 0 {
			t.Units = t.Amount / t.NAV
		}
		d, _ := time.Parse("2006-01-02", t.Date)
		switch t.Kind {
		case "subscription", "transfer_in":
			h.Units += t.Units
			h.Invested += t.Amount
			if t.Amount != 0 {
				cfs = append(cfs, CashFlow{Date: d, Amount: -t.Amount})
			}
		case "redemption", "transfer_out":
			h.Units -= t.Units
			h.Invested -= t.Amount
			if t.Amount != 0 {
				cfs = append(cfs, CashFlow{Date: d, Amount: t.Amount})
			}
		}
		if h.Since == "" || t.Date < h.Since {
			h.Since = t.Date
		}
		h.Txns = append(h.Txns, t)
	}
	if n := len(series); n > 0 {
		last := series[n-1]
		h.Value = h.Units * last.NAV
		if h.Invested > 0 {
			h.Return = h.Value/h.Invested - 1
		}
		d, _ := time.Parse("2006-01-02", last.Date)
		if len(cfs) > 0 && h.Value > 0 {
			if first := cfs[0].Date; d.Sub(first) >= 30*24*time.Hour {
				if x, ok := XIRR(append(cfs, CashFlow{Date: d, Amount: h.Value})); ok {
					h.XIRR = &x
				}
			}
		}
	}
	return h, nil
}

// InvestorFor finds the investor record for a sign-in email.
func (s *Service) InvestorFor(email string) *core.Record {
	r, err := s.app.FindFirstRecordByFilter("investors", "email = {:e} && inactive = false", map[string]any{"e": strings.ToLower(strings.TrimSpace(email))})
	if err != nil {
		return nil
	}
	return r
}

// ---------- portfolio (signed in) ----------

type SeriesPoint struct {
	D    string   `json:"d"`
	NAV  float64  `json:"nav"`
	N50  *float64 `json:"n50,omitempty"`
	N500 *float64 `json:"n500,omitempty"`
}

type PortfolioView struct {
	AsOf       string        `json:"asOf"`
	Inception  string        `json:"inception"`
	LastSync   *time.Time    `json:"lastSync"`
	NAV        float64       `json:"nav"`
	NAVChange  float64       `json:"navChange"`
	StartNAV   float64       `json:"startNav"`
	AUM        float64       `json:"aum"`
	Units      float64       `json:"units"`
	Series     []SeriesPoint `json:"series"`
	Risk       Risk          `json:"risk"`   // vs the Nifty 500 (main benchmark)
	Risk50     Risk          `json:"risk50"` // vs the Nifty 50
	Positions  []Position    `json:"positions"`
	Cash       float64       `json:"cash"`
	CashWeight float64       `json:"cashWeight"`
	Sectors    []Sector      `json:"sectors"`
	Pods       []PodStat     `json:"pods"`
	Monthly    []MonthRow    `json:"monthly"`
	Realised   float64       `json:"realised"`
	Settings   Settings      `json:"settings"`
	Me         *Holding      `json:"me"`
	Warnings   []string      `json:"warnings,omitempty"`
}

func (s *Service) Portfolio(user *core.Record, admin bool) (*PortfolioView, error) {
	sn := s.Snap()
	res := sn.Result
	v := &PortfolioView{AsOf: res.AsOf, Inception: res.Inception, StartNAV: res.StartNAV, Risk: sn.Risk, Risk50: sn.Risk50, Positions: res.Positions,
		Sectors: sn.Sectors, Pods: res.Pods, Monthly: sn.Monthly, Realised: res.Realised + res.Other, Settings: sn.Settings, Series: []SeriesPoint{}}
	if v.Positions == nil {
		v.Positions = []Position{}
	}
	if !sn.LastSync.IsZero() {
		t := sn.LastSync
		v.LastSync = &t
	}
	if n := len(res.Series); n > 0 {
		last := res.Series[n-1]
		v.NAV, v.AUM, v.Units, v.Cash = last.NAV, last.Value, last.Units, last.Cash
		if last.Value > 0 {
			v.CashWeight = last.Cash / last.Value
		}
		if n > 1 {
			v.NAVChange = last.NAV/res.Series[n-2].NAV - 1
		}
		for _, p := range res.Series {
			sp := SeriesPoint{D: p.Date, NAV: round(p.NAV, 4)}
			if p.Nifty500 > 0 {
				n500 := p.Nifty500
				sp.N500 = &n500
			}
			if p.Nifty50 > 0 {
				n50 := p.Nifty50
				sp.N50 = &n50
			}
			v.Series = append(v.Series, sp)
		}
	}
	if inv := s.InvestorFor(user.GetString("email")); inv != nil {
		h, err := s.holding(inv, sn)
		if err != nil {
			return nil, err
		}
		v.Me = h
	}
	if admin {
		v.Warnings = res.Warnings
	} else {
		// Pod-level books are for fund admins only: drop them before they leave the server.
		v.Pods = []PodStat{}
		pos := make([]Position, len(v.Positions))
		for i, p := range v.Positions {
			p.Pods = nil
			pos[i] = p
		}
		v.Positions = pos
	}
	return v, nil
}

// ---------- reports ----------

type ReportMeta struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Category  string `json:"category"`
	Date      string `json:"date"`
	Access    string `json:"access"`
	Author    string `json:"author"`
	Dek       string `json:"dek"`
	Summary   string `json:"summary"`
	Kicker    string `json:"kicker"`
	KickerSub string `json:"kickerSub"`
	Pages     string `json:"pages"`
	ReadMins  int    `json:"readMins"`
	HasPDF    bool   `json:"hasPdf"`
	Locked    bool   `json:"locked"`
}

type Report struct {
	ReportMeta
	Facts   any         `json:"facts"`
	Body    string      `json:"body"`
	Preview bool        `json:"preview"`
	TOC     []string    `json:"toc"`
	Next    *ReportMeta `json:"next"`
}

func reportMeta(r *core.Record, authed bool) ReportMeta {
	body := r.GetString("body")
	words := len(strings.Fields(body))
	m := ReportMeta{Slug: r.GetString("slug"), Title: r.GetString("title"), Type: r.GetString("type"), Category: r.GetString("category"),
		Date: r.GetString("date"), Access: r.GetString("access"), Author: r.GetString("author"), Dek: r.GetString("dek"),
		Summary: r.GetString("summary"), Kicker: r.GetString("kicker"), KickerSub: r.GetString("kickerSub"), Pages: r.GetString("pages"),
		ReadMins: int(math.Max(1, math.Round(float64(words)/220))), HasPDF: r.GetString("pdf") != ""}
	m.Locked = m.Access == "investors" && !authed
	return m
}

func (s *Service) reportRecords() ([]*core.Record, error) {
	return s.app.FindRecordsByFilter("reports", "published = true", "-date,-created", 0, 0)
}

func (s *Service) Reports(authed bool) ([]ReportMeta, error) {
	rows, err := s.reportRecords()
	if err != nil {
		return nil, err
	}
	out := []ReportMeta{}
	for _, r := range rows {
		out = append(out, reportMeta(r, authed))
	}
	return out, nil
}

var headingLine = regexp.MustCompile(`(?m)^##\s+(.+?)\s*$`)

// PreviewBlocks is how much of an investors-only report the public may read.
const PreviewBlocks = 3

func (s *Service) Report(slug string, authed bool) (*Report, *core.Record, error) {
	rows, err := s.reportRecords()
	if err != nil {
		return nil, nil, err
	}
	for i, r := range rows {
		if r.GetString("slug") != slug {
			continue
		}
		rep := &Report{ReportMeta: reportMeta(r, authed), Facts: r.Get("facts"), Body: r.GetString("body"), TOC: []string{}}
		for _, m := range headingLine.FindAllStringSubmatch(rep.Body, -1) {
			rep.TOC = append(rep.TOC, m[1])
		}
		if rep.Locked {
			rep.Body, rep.Preview = previewOf(rep.Body, PreviewBlocks), true
		}
		if len(rows) > 1 {
			n := reportMeta(rows[(i+1)%len(rows)], authed)
			rep.Next = &n
		}
		return rep, r, nil
	}
	return nil, nil, nil
}

// previewOf keeps the first n blocks (paragraphs, headings, quotes) of a Markdown body.
func previewOf(body string, n int) string {
	blocks := regexp.MustCompile(`\n\s*\n`).Split(strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n")), -1)
	if len(blocks) > n {
		blocks = blocks[:n]
	}
	for len(blocks) > 1 && strings.HasPrefix(strings.TrimSpace(blocks[len(blocks)-1]), "#") {
		blocks = blocks[:len(blocks)-1] // don't end on a heading
	}
	return strings.Join(blocks, "\n\n")
}

// ---------- team ----------

type Member struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Group    string `json:"group"`
	Org      string `json:"org"`
	Batch    string `json:"batch"`
	Photo    string `json:"photo"`
	LinkedIn string `json:"linkedin"`
}

func (s *Service) Team() ([]Member, error) {
	rows, err := s.app.FindRecordsByFilter("team", "", "order,name", 0, 0)
	if err != nil {
		return nil, err
	}
	out := []Member{}
	for _, r := range rows {
		m := Member{ID: r.Id, Name: r.GetString("name"), Role: r.GetString("role"), Group: r.GetString("group"), Org: r.GetString("org"),
			Batch: r.GetString("batch"), LinkedIn: r.GetString("linkedin")}
		if f := r.GetString("photo"); f != "" {
			m.Photo = "/api/files/" + r.Collection().Id + "/" + r.Id + "/" + f
		}
		out = append(out, m)
	}
	return out, nil
}

// ---------- admin ----------

type SyncRow struct {
	At       time.Time `json:"at"`
	Source   string    `json:"source"`
	OK       bool      `json:"ok"`
	Message  string    `json:"message"`
	Trades   int       `json:"trades"`
	Warnings any       `json:"warnings"`
	By       string    `json:"by"`
}

type AdminStatus struct {
	FlowsFromInvestors bool           `json:"flowsFromInvestors"`
	Flows              []FlowRow      `json:"flows"`
	ExcelURL           bool           `json:"excelUrl"`
	PriceHistory       bool           `json:"priceHistory"`
	Syncs              []SyncRow      `json:"syncs"`
	Counts             map[string]int `json:"counts"`
	FlowUnits          float64        `json:"flowUnits"`
	FlowAmount         float64        `json:"flowAmount"`
	InvestorUnits      float64        `json:"investorUnits"`
	Investors          []Holding      `json:"investors"`
	Warnings           []string       `json:"warnings"`
	NoHistory          []string       `json:"noHistory"`
	AsOf               string         `json:"asOf"`
	NAV                float64        `json:"nav"`
}

func (s *Service) Admin() (*AdminStatus, error) {
	sn := s.Snap()
	st := &AdminStatus{ExcelURL: s.ExcelURL != "", PriceHistory: s.Yahoo != nil, Counts: map[string]int{}, Syncs: []SyncRow{},
		Investors: []Holding{}, Warnings: sn.Result.Warnings, NoHistory: []string{}, AsOf: sn.Result.AsOf}
	if n := len(sn.Result.Series); n > 0 {
		st.NAV = sn.Result.Series[n-1].NAV
		st.FlowUnits = sn.Result.Series[n-1].Units
	}
	rows, err := s.app.FindRecordsByFilter("sync_log", "", "-created", 15, 0)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		st.Syncs = append(st.Syncs, SyncRow{At: r.GetDateTime("created").Time(), Source: r.GetString("source"), OK: r.GetBool("ok"),
			Message: r.GetString("message"), Trades: r.GetInt("trades"), Warnings: r.Get("warnings"), By: r.GetString("by")})
	}
	for _, c := range []string{"trades", "instruments", "prices", "investors", "investor_txns", "capital_flows", "reports", "team"} {
		n, _ := s.app.CountRecords(c)
		st.Counts[c] = int(n)
	}
	flows, err := s.fundFlows()
	if err != nil {
		return nil, err
	}
	for _, f := range flows {
		st.FlowAmount += f.Amount
	}
	n, _ := s.app.CountRecords("capital_flows")
	st.FlowsFromInvestors = n == 0
	if st.Flows, err = s.Flows(); err != nil {
		return nil, err
	}
	invs, err := s.app.FindRecordsByFilter("investors", "", "name", 0, 0)
	if err != nil {
		return nil, err
	}
	for _, inv := range invs {
		h, err := s.holding(inv, sn)
		if err != nil {
			return nil, err
		}
		st.InvestorUnits += h.Units
		st.Investors = append(st.Investors, *h)
	}
	// Instruments we hold with no downloaded history.
	var miss []struct {
		Symbol string `db:"symbol"`
	}
	_ = s.app.DB().NewQuery("SELECT i.symbol FROM instruments i WHERE NOT EXISTS (SELECT 1 FROM prices p WHERE p.symbol = i.symbol AND p.source IN ('yahoo','manual')) ORDER BY i.symbol").All(&miss)
	for _, m := range miss {
		st.NoHistory = append(st.NoHistory, m.Symbol)
	}
	return st, nil
}
