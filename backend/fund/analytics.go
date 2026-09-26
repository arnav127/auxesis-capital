package fund

import (
	"math"
	"sort"
	"time"
)

// Risk is computed from daily NAV and Nifty 50 closes since the cycle began.
type Risk struct {
	Days         int     `json:"days"`
	FundReturn   float64 `json:"fundReturn"`  // since the cycle began
	BenchReturn  float64 `json:"benchReturn"` // Nifty 50, same period
	Bench500     float64 `json:"bench500Return"`
	CAGR         float64 `json:"cagr"`
	BenchCAGR    float64 `json:"benchCagr"`
	Annualised   bool    `json:"annualised"` // false when under a year: CAGR equals the plain return
	Alpha        float64 `json:"alpha"`      // Jensen's alpha, annualised
	Beta         float64 `json:"beta"`
	Volatility   float64 `json:"volatility"`
	Sharpe       float64 `json:"sharpe"`
	Sortino      float64 `json:"sortino"`
	Information  float64 `json:"information"`
	TrackingErr  float64 `json:"trackingError"`
	MaxDrawdown  float64 `json:"maxDrawdown"`
	UpCapture    float64 `json:"upCapture"`
	DownCapture  float64 `json:"downCapture"`
	RiskFree     float64 `json:"riskFree"`
	HasBenchmark bool    `json:"hasBenchmark"`
}

// Analyse computes risk statistics. rf is the annual risk-free rate (e.g. 0.065).
func Analyse(s []NavPoint, startNAV, rf float64) Risk {
	r := Risk{RiskFree: rf}
	if len(s) < 2 {
		return r
	}
	last := s[len(s)-1]
	if startNAV <= 0 {
		startNAV = s[0].NAV
	}
	r.FundReturn = last.NAV/startNAV - 1
	// Day one is measured from the issue price; the benchmark has no prior close to compare.
	rfund := []float64{s[0].NAV/startNAV - 1}
	rbench := []float64{math.NaN()}
	for i := 1; i < len(s); i++ {
		rfund = append(rfund, s[i].NAV/s[i-1].NAV-1)
		if s[i].Nifty50 > 0 && s[i-1].Nifty50 > 0 {
			rbench = append(rbench, s[i].Nifty50/s[i-1].Nifty50-1)
		} else {
			rbench = append(rbench, math.NaN())
		}
	}
	r.Days = len(rfund)
	b0, b1 := firstPos(s, func(p NavPoint) float64 { return p.Nifty50 }), last.Nifty50
	if b0 > 0 && b1 > 0 {
		r.BenchReturn = b1/b0 - 1
		r.HasBenchmark = true
	}
	if c0 := firstPos(s, func(p NavPoint) float64 { return p.Nifty500 }); c0 > 0 && last.Nifty500 > 0 {
		r.Bench500 = last.Nifty500/c0 - 1
	}

	years := float64(len(rfund)) / 252
	r.Annualised = years >= 1
	if r.Annualised {
		r.CAGR = math.Pow(1+r.FundReturn, 1/years) - 1
		r.BenchCAGR = math.Pow(1+r.BenchReturn, 1/years) - 1
	} else {
		r.CAGR, r.BenchCAGR = r.FundReturn, r.BenchReturn
	}

	mf := mean(rfund)
	var vf, dd float64
	for _, x := range rfund {
		vf += (x - mf) * (x - mf)
		if x < 0 {
			dd += x * x
		}
	}
	n := float64(len(rfund))
	r.Volatility = math.Sqrt(vf / n * 252)
	down := math.Sqrt(dd / n * 252)
	// Sharpe and Sortino use the mean daily excess return, annualised, so short histories stay sensible.
	exAnn := mf*252 - rf
	if r.Volatility > 0 {
		r.Sharpe = exAnn / r.Volatility
	}
	if down > 0 {
		r.Sortino = exAnn / down
	}

	peak := 0.0
	for _, p := range s {
		peak = math.Max(peak, p.NAV)
		r.MaxDrawdown = math.Min(r.MaxDrawdown, p.NAV/peak-1)
	}

	if r.HasBenchmark {
		var xs, ys []float64
		for i := range rfund {
			if !math.IsNaN(rbench[i]) {
				xs, ys = append(xs, rbench[i]), append(ys, rfund[i])
			}
		}
		if len(xs) > 2 {
			mb, my := mean(xs), mean(ys)
			var cov, vb, te float64
			ex := make([]float64, len(xs))
			for i := range xs {
				cov += (xs[i] - mb) * (ys[i] - my)
				vb += (xs[i] - mb) * (xs[i] - mb)
				ex[i] = ys[i] - xs[i]
			}
			mex := mean(ex)
			for _, e := range ex {
				te += (e - mex) * (e - mex)
			}
			if vb > 0 {
				r.Beta = cov / vb
			}
			r.TrackingErr = math.Sqrt(te / float64(len(ex)) * 252)
			if r.TrackingErr > 0 {
				r.Information = mex * 252 / r.TrackingErr
			}
			// Jensen's alpha on annualised mean daily returns.
			r.Alpha = (my*252 - rf) - r.Beta*(mb*252-rf)
			var up, upB, dn, dnB float64
			for i := range xs {
				if xs[i] >= 0 {
					up, upB = up+ys[i], upB+xs[i]
				} else {
					dn, dnB = dn+ys[i], dnB+xs[i]
				}
			}
			if upB != 0 {
				r.UpCapture = up / upB
			}
			if dnB != 0 {
				r.DownCapture = dn / dnB
			}
		}
	}
	return r
}

func firstPos(s []NavPoint, f func(NavPoint) float64) float64 {
	for _, p := range s {
		if v := f(p); v > 0 {
			return v
		}
	}
	return 0
}

func mean(a []float64) float64 {
	if len(a) == 0 {
		return 0
	}
	var t float64
	for _, x := range a {
		t += x
	}
	return t / float64(len(a))
}

// MonthRow is one year of monthly NAV returns.
type MonthRow struct {
	Year   int          `json:"year"`
	Months [12]*float64 `json:"months"`
	Year1  float64      `json:"ytd"`
}

// Monthly returns month-end to month-end (the first month from inception).
func Monthly(s []NavPoint, startNAV float64) []MonthRow {
	if len(s) == 0 {
		return nil
	}
	type ym struct{ y, m int }
	lastOf := map[ym]float64{}
	var keys []ym
	for _, p := range s {
		t, _ := time.Parse("2006-01-02", p.Date)
		k := ym{t.Year(), int(t.Month())}
		if _, ok := lastOf[k]; !ok {
			keys = append(keys, k)
		}
		lastOf[k] = p.NAV
	}
	rows := map[int]*MonthRow{}
	prev := startNAV
	if prev <= 0 {
		prev = s[0].NAV
	}
	yearStart := map[int]float64{}
	for _, k := range keys {
		if rows[k.y] == nil {
			rows[k.y] = &MonthRow{Year: k.y}
			yearStart[k.y] = prev
		}
		v := lastOf[k]/prev - 1
		rows[k.y].Months[k.m-1] = &v
		rows[k.y].Year1 = lastOf[k]/yearStart[k.y] - 1
		prev = lastOf[k]
	}
	var out []MonthRow
	for _, r := range rows {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Year < out[j].Year })
	return out
}

// CashFlow for XIRR: negative = paid in, positive = received / current value.
type CashFlow struct {
	Date   time.Time
	Amount float64
}

// XIRR solves for the annual rate that sets the flows' present value to zero.
func XIRR(flows []CashFlow) (float64, bool) {
	if len(flows) < 2 {
		return 0, false
	}
	t0 := flows[0].Date
	for _, f := range flows {
		if f.Date.Before(t0) {
			t0 = f.Date
		}
	}
	npv := func(r float64) (float64, float64) {
		var v, dv float64
		for _, f := range flows {
			y := f.Date.Sub(t0).Hours() / 24 / 365
			d := math.Pow(1+r, y)
			v += f.Amount / d
			dv -= y * f.Amount / (d * (1 + r))
		}
		return v, dv
	}
	r := 0.1
	for i := 0; i < 100; i++ {
		v, dv := npv(r)
		if math.Abs(v) < 1e-7 {
			return r, true
		}
		if dv == 0 {
			break
		}
		nr := r - v/dv
		if nr <= -0.9999 {
			nr = (r - 0.9999) / 2
		}
		if math.Abs(nr-r) < 1e-10 {
			return nr, true
		}
		r = nr
	}
	// Fall back to bisection.
	lo, hi := -0.9999, 10.0
	vlo, _ := npv(lo)
	vhi, _ := npv(hi)
	if vlo*vhi > 0 {
		return 0, false
	}
	for i := 0; i < 200; i++ {
		mid := (lo + hi) / 2
		v, _ := npv(mid)
		if v*vlo > 0 {
			lo, vlo = mid, v
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2, true
}
