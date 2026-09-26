package fund

import (
	"math"
	"sort"
	"time"
)

// The NAV engine. Given the trade log, the fund's capital flows and daily closing prices,
// it marks the portfolio to market on every trading day:
//
//	cash      = capital in − capital out − Σ (qty × trade price) + other realised P&L
//	holdings  = Σ open qty × that day's close
//	NAV/unit  = (cash + holdings) / units in issue
//
// New capital buys units at the previous close's NAV (the first flow at StartNAV).
// Brokerage is not in the trade log, so NAV is before charges on the equity book.

// Day is a calendar date in IST, "2006-01-02".
type Day = string

func DayOf(t time.Time) Day { return t.In(IST).Format("2006-01-02") }

// dateDay reads a date that has no time zone (Excel dates are stored as midnight UTC).
func dateDay(t time.Time) Day { return t.UTC().Format("2006-01-02") }

var IST = time.FixedZone("IST", 5*3600+1800)

// Flow is money into (+) or out of (−) the fund on a date.
type Flow struct {
	Date   Day     `json:"date"`
	Amount float64 `json:"amount"`
	Units  float64 `json:"units"` // 0: priced at the previous day's NAV
	Note   string  `json:"note"`
}

// Prices maps symbol -> day -> close.
type Prices map[string]map[Day]float64

func (p Prices) Set(sym string, d Day, v float64) {
	if p[sym] == nil {
		p[sym] = map[Day]float64{}
	}
	p[sym][d] = v
}

// series gives O(log n) "last close on or before day" lookups.
type series struct {
	days []Day
	vals []float64
}

func (p Prices) index() map[string]*series {
	out := map[string]*series{}
	for sym, m := range p {
		s := &series{}
		for d := range m {
			s.days = append(s.days, d)
		}
		sort.Strings(s.days)
		for _, d := range s.days {
			s.vals = append(s.vals, m[d])
		}
		out[sym] = s
	}
	return out
}

func (s *series) at(d Day) (float64, Day, bool) {
	if s == nil {
		return 0, "", false
	}
	i := sort.SearchStrings(s.days, d)
	if i < len(s.days) && s.days[i] == d {
		return s.vals[i], d, true
	}
	if i == 0 {
		return 0, "", false
	}
	return s.vals[i-1], s.days[i-1], true
}

// NavPoint is the fund at one day's close.
type NavPoint struct {
	Date      Day     `json:"date"`
	NAV       float64 `json:"nav"`
	Cash      float64 `json:"cash"`
	Holdings  float64 `json:"holdings"`
	Value     float64 `json:"value"`
	Units     float64 `json:"units"`
	Nifty50   float64 `json:"nifty50"`  // index level (0 if unknown)
	Nifty500  float64 `json:"nifty500"` // index level (0 if unknown)
	Positions int     `json:"positions"`
}

// Position is an open holding across all pods.
type Position struct {
	Symbol    string   `json:"symbol"`
	Name      string   `json:"name"`
	Sector    string   `json:"sector"`
	Pods      []string `json:"pods"`
	Qty       float64  `json:"qty"`
	AvgCost   float64  `json:"avgCost"` // accounting cost of the shares held (resets when a position is closed)
	AvgBuy    float64  `json:"avgBuy"`  // average price of every buy this cycle, weighted by quantity
	Price     float64  `json:"price"`
	PrevClose float64  `json:"prevClose"`
	PriceDate Day      `json:"priceDate"`
	Stale     bool     `json:"stale"` // no market price; marked at the last trade price
	Value     float64  `json:"value"`
	Weight    float64  `json:"weight"`
	Day1      float64  `json:"day1"`   // today's change
	Return    float64  `json:"return"` // current price vs average buy price
	Unreal    float64  `json:"unrealised"`
	Since     Day      `json:"since"`
}

// PodStat is one pod's book.
type PodStat struct {
	Pod        string  `json:"pod"`
	Trades     int     `json:"trades"`
	Open       int     `json:"open"`
	Invested   float64 `json:"invested"` // cost of open positions
	Value      float64 `json:"value"`
	Realised   float64 `json:"realised"`
	Unrealised float64 `json:"unrealised"`
	PnL        float64 `json:"pnl"`
}

// Result is the whole fund, marked to AsOf.
// Realisation is the profit or loss booked on one stock this cycle (average-cost accounting).
type Realisation struct {
	Symbol    string   `json:"symbol"`
	Name      string   `json:"name"`
	Sector    string   `json:"sector"`
	Pods      []string `json:"pods,omitempty"`
	QtySold   float64  `json:"qtySold"`
	AvgCost   float64  `json:"avgCost"` // average cost of the shares sold
	AvgSell   float64  `json:"avgSell"` // average price they were sold at
	Cost      float64  `json:"cost"`
	Proceeds  float64  `json:"proceeds"`
	PnL       float64  `json:"pnl"`
	Return    float64  `json:"return"` // PnL / cost
	Sells     int      `json:"sells"`
	LastSold  Day      `json:"lastSold"`
	StillHeld bool     `json:"stillHeld"`
}

// OtherBook is realised P&L that isn't a stock trade (e.g. the Sensex 0DTE options book).
type OtherBook struct {
	Book    string  `json:"book"`
	Trades  int     `json:"trades"`
	Gross   float64 `json:"gross"`
	Charges float64 `json:"charges"`
	Net     float64 `json:"net"`
}

type Result struct {
	AsOf      Day        `json:"asOf"`
	Inception Day        `json:"inception"`
	StartNAV  float64    `json:"startNav"`
	Series    []NavPoint `json:"series"`
	Positions []Position `json:"positions"`
	Pods      []PodStat  `json:"pods"`
	Realised  float64    `json:"realised"`
	// Realised P&L by stock (largest profit first) and by other book.
	Realisations []Realisation `json:"realisations"`
	OtherBooks   []OtherBook   `json:"otherBooks"`
	Other        float64       `json:"other"`
	Warnings     []string      `json:"warnings"`
}

type book struct {
	qty, cost, realised float64
	since               Day
	// What has been closed so far: quantity, its cost at average cost, and what it fetched.
	closedQty, closedCost, closedProceeds float64
	closes                                int
	lastClose                             Day
}

// apply adds a trade with average-cost accounting; returns realised P&L.
func (b *book) apply(q, p float64, d Day) float64 {
	var realised float64
	if b.qty != 0 && (b.qty > 0) != (q > 0) { // reducing (or flipping) the position
		avg := b.cost / b.qty
		closing := math.Min(math.Abs(q), math.Abs(b.qty)) * sign(q) // same sign as q
		realised = -closing * (p - avg)                             // selling +qty at p: (p-avg)*|closing|
		n := math.Abs(closing)
		if b.qty > 0 { // selling out of a long
			b.closedCost += n * avg
			b.closedProceeds += n * p
		} else { // buying back a short
			b.closedCost += n * p
			b.closedProceeds += n * avg
		}
		b.closedQty += n
		b.closes++
		b.lastClose = d
		b.cost -= -closing * avg
		b.qty += closing
		q -= closing
		if math.Abs(b.qty) < 1e-9 {
			b.qty, b.cost, b.since = 0, 0, ""
		}
	}
	if q != 0 {
		if b.qty == 0 {
			b.since = d
		}
		b.qty += q
		b.cost += q * p
	}
	b.realised += realised
	return realised
}

func sign(x float64) float64 {
	if x < 0 {
		return -1
	}
	return 1
}

// Calendar returns the trading days from start to end: the days the Nifty 50 (or Nifty 500)
// has a close, or weekdays if we have no index history.
func Calendar(prices Prices, start, end Day) []Day {
	var days []Day
	m := prices[Nifty50]
	if len(prices[Nifty500]) > len(m) {
		m = prices[Nifty500]
	}
	if len(m) > 5 {
		for d := range m {
			if d >= start && d <= end {
				days = append(days, d)
			}
		}
		sort.Strings(days)
		// Days after the last index close (e.g. today's close not fetched yet) still count.
		last := start
		if len(days) > 0 {
			last = days[len(days)-1]
		}
		for _, d := range weekdays(last, end) {
			if d > last {
				days = append(days, d)
			}
		}
		if len(days) == 0 || days[0] != start {
			days = append([]Day{start}, days...)
			sort.Strings(days)
		}
		return days
	}
	return weekdays(start, end)
}

func weekdays(start, end Day) []Day {
	s, err1 := time.Parse("2006-01-02", start)
	e, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil {
		return nil
	}
	var out []Day
	for d := s; !d.After(e); d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, d.Format("2006-01-02"))
		}
	}
	return out
}

// Compute marks the fund to market on every day of the calendar up to asOf.
func Compute(trades []Trade, other []PnLEntry, flows []Flow, prices Prices, startNAV float64, asOf Day, sectors map[string]string) *Result {
	if startNAV <= 0 {
		startNAV = 1000
	}
	res := &Result{StartNAV: startNAV, AsOf: asOf}
	sort.SliceStable(trades, func(i, j int) bool {
		if !trades[i].Date.Equal(trades[j].Date) {
			return trades[i].Date.Before(trades[j].Date)
		}
		return trades[i].Row < trades[j].Row
	})
	sort.SliceStable(flows, func(i, j int) bool { return flows[i].Date < flows[j].Date })

	inception := ""
	if len(flows) > 0 {
		inception = flows[0].Date
	}
	if len(trades) > 0 && (inception == "" || dateDay(trades[0].Date) < inception) {
		inception = dateDay(trades[0].Date)
	}
	if inception == "" {
		return res
	}
	res.Inception = inception
	if asOf < inception {
		asOf = inception
		res.AsOf = asOf
	}

	idx := prices.index()
	days := Calendar(prices, inception, asOf)

	fundBooks := map[string]*book{}
	podBooks := map[[2]string]*book{}
	podTrades := map[string]int{}
	names, lastTradePx := map[string]string{}, map[string]float64{}
	buyQty, buyCost := map[string]float64{}, map[string]float64{}

	cash, units, prevNAV := 0.0, 0.0, startNAV
	otherBooks := map[string]*OtherBook{}
	ti, fi, oi := 0, 0, 0
	sort.SliceStable(other, func(i, j int) bool { return other[i].Date.Before(other[j].Date) })

	mark := func(sym string, d Day) (float64, Day, bool) {
		if v, pd, ok := idx[sym].at(d); ok && v > 0 {
			return v, pd, true
		}
		if v, ok := lastTradePx[sym]; ok {
			return v, "", false
		}
		return 0, "", false
	}

	for _, d := range days {
		for fi < len(flows) && flows[fi].Date <= d {
			f := flows[fi]
			u := f.Units
			if u == 0 {
				u = f.Amount / prevNAV
			}
			cash += f.Amount
			units += u
			fi++
		}
		for ti < len(trades) && dateDay(trades[ti].Date) <= d {
			t := trades[ti]
			if fundBooks[t.Symbol] == nil {
				fundBooks[t.Symbol] = &book{}
			}
			res.Realised += fundBooks[t.Symbol].apply(t.Qty, t.Price, dateDay(t.Date))
			k := [2]string{t.Pod, t.Symbol}
			if podBooks[k] == nil {
				podBooks[k] = &book{}
			}
			podBooks[k].apply(t.Qty, t.Price, dateDay(t.Date))
			podTrades[t.Pod]++
			cash -= t.Qty * t.Price
			names[t.Symbol], lastTradePx[t.Symbol] = t.Name, t.Price
			if t.Qty > 0 {
				buyQty[t.Symbol] += t.Qty
				buyCost[t.Symbol] += t.Qty * t.Price
			}
			ti++
		}
		for oi < len(other) && dateDay(other[oi].Date) <= d {
			cash += other[oi].Net
			res.Other += other[oi].Net
			ob := otherBooks[other[oi].Book]
			if ob == nil {
				ob = &OtherBook{Book: other[oi].Book}
				otherBooks[other[oi].Book] = ob
			}
			ob.Trades++
			ob.Gross += other[oi].Gross
			ob.Charges += other[oi].Charges
			ob.Net += other[oi].Net
			oi++
		}
		holdings, open := 0.0, 0
		for sym, b := range fundBooks {
			if b.qty == 0 {
				continue
			}
			px, _, _ := mark(sym, d)
			holdings += b.qty * px
			open++
		}
		value := cash + holdings
		nav := startNAV
		if units > 0 {
			nav = value / units
		}
		p := NavPoint{Date: d, NAV: nav, Cash: cash, Holdings: holdings, Value: value, Units: units, Positions: open}
		p.Nifty50, _, _ = idx[Nifty50].at(d)
		p.Nifty500, _, _ = idx[Nifty500].at(d)
		res.Series = append(res.Series, p)
		prevNAV = nav
	}

	// Open positions at asOf.
	var total float64
	if n := len(res.Series); n > 0 {
		total = res.Series[n-1].Value
	}
	podsBySym := map[string][]string{}
	for k, b := range podBooks {
		if b.qty != 0 {
			podsBySym[k[1]] = append(podsBySym[k[1]], k[0])
		}
	}
	for sym, b := range fundBooks {
		if b.qty == 0 {
			continue
		}
		px, pd, ok := mark(sym, asOf)
		prev := px
		if ok {
			if pd2 := prevDay(idx[sym], pd); pd2 > 0 {
				prev = pd2
			}
		}
		pos := Position{Symbol: sym, Name: names[sym], Qty: b.qty, AvgCost: b.cost / b.qty, Price: px, PrevClose: prev,
			PriceDate: pd, Stale: !ok, Value: b.qty * px, Since: b.since, Pods: podsBySym[sym]}
		if s := sectors[sym]; s != "" {
			pos.Sector = s
		}
		if pos.Sector == "" {
			pos.Sector = "Other"
		}
		sort.Strings(pos.Pods)
		if total > 0 {
			pos.Weight = pos.Value / total
		}
		if prev > 0 {
			pos.Day1 = px/prev - 1
		}
		if buyQty[sym] > 0 {
			pos.AvgBuy = buyCost[sym] / buyQty[sym]
		}
		if pos.AvgBuy > 0 {
			pos.Return = px/pos.AvgBuy - 1
		}
		pos.Unreal = pos.Value - b.cost
		res.Positions = append(res.Positions, pos)
	}
	sort.Slice(res.Positions, func(i, j int) bool { return res.Positions[i].Value > res.Positions[j].Value })

	pods := map[string]*PodStat{}
	for k, b := range podBooks {
		ps := pods[k[0]]
		if ps == nil {
			ps = &PodStat{Pod: k[0], Trades: podTrades[k[0]]}
			pods[k[0]] = ps
		}
		ps.Realised += b.realised
		if b.qty != 0 {
			px, _, _ := mark(k[1], asOf)
			ps.Open++
			ps.Invested += b.cost
			ps.Value += b.qty * px
			ps.Unrealised += b.qty*px - b.cost
		}
	}
	for _, ps := range pods {
		ps.PnL = ps.Realised + ps.Unrealised
		res.Pods = append(res.Pods, *ps)
	}

	// Realised P&L by stock.
	podsClosed := map[string][]string{}
	for k, b := range podBooks {
		if b.closedQty > 0 {
			podsClosed[k[1]] = append(podsClosed[k[1]], k[0])
		}
	}
	res.Realisations = []Realisation{}
	for sym, b := range fundBooks {
		if b.closedQty <= 0 {
			continue
		}
		r := Realisation{Symbol: sym, Name: names[sym], Sector: sectors[sym], Pods: podsClosed[sym], QtySold: b.closedQty,
			AvgCost: b.closedCost / b.closedQty, AvgSell: b.closedProceeds / b.closedQty, Cost: b.closedCost, Proceeds: b.closedProceeds,
			PnL: b.realised, Sells: b.closes, LastSold: b.lastClose, StillHeld: b.qty != 0}
		if r.Sector == "" {
			r.Sector = "Other"
		}
		if r.Cost > 0 {
			r.Return = r.PnL / r.Cost
		}
		sort.Strings(r.Pods)
		res.Realisations = append(res.Realisations, r)
	}
	sort.Slice(res.Realisations, func(i, j int) bool { return res.Realisations[i].PnL > res.Realisations[j].PnL })
	res.OtherBooks = []OtherBook{}
	for _, ob := range otherBooks {
		res.OtherBooks = append(res.OtherBooks, *ob)
	}
	sort.Slice(res.OtherBooks, func(i, j int) bool { return res.OtherBooks[i].Book < res.OtherBooks[j].Book })
	sort.Slice(res.Pods, func(i, j int) bool { return res.Pods[i].Pod < res.Pods[j].Pod })

	for _, p := range res.Positions {
		if p.Stale {
			res.Warnings = append(res.Warnings, "No market price for "+p.Symbol+" ("+p.Name+"): it is valued at its last trade price.")
		}
	}
	return res
}

func prevDay(s *series, d Day) float64 {
	if s == nil {
		return 0
	}
	i := sort.SearchStrings(s.days, d)
	if i > 0 && i <= len(s.days) {
		return s.vals[i-1]
	}
	return 0
}
