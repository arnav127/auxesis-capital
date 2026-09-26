package fund

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Yahoo fetches daily closes from Yahoo Finance's chart API (no key needed).
// NSE stocks are <SYMBOL>.NS; the benchmarks are ^NSEI (Nifty 50) and ^CRSLDX (Nifty 500).
type Yahoo struct {
	Client  *http.Client
	BaseURL string // default https://query1.finance.yahoo.com
}

// YahooSymbol maps our symbol to Yahoo's. override wins when set.
func YahooSymbol(sym, override string) string {
	if override != "" {
		return override
	}
	switch sym {
	case Nifty50:
		return "^NSEI"
	case Nifty500:
		return "^CRSLDX"
	}
	return sym + ".NS"
}

// Closes returns day -> close between from and to (inclusive, IST days).
func (y *Yahoo) Closes(ctx context.Context, ysym string, from, to time.Time) (map[Day]float64, error) {
	base := y.BaseURL
	if base == "" {
		base = "https://query1.finance.yahoo.com"
	}
	c := y.Client
	if c == nil {
		c = &http.Client{Timeout: 20 * time.Second}
	}
	u := fmt.Sprintf("%s/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d&events=history&includeAdjustedClose=false",
		base, url.PathEscape(ysym), from.Add(-24*time.Hour).Unix(), to.Add(24*time.Hour).Unix())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var body struct {
		Chart struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Close []*float64 `json:"close"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
			Error *struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		} `json:"chart"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("%s: HTTP %d, unreadable reply", ysym, res.StatusCode)
	}
	if body.Chart.Error != nil {
		return nil, fmt.Errorf("%s: %s", ysym, strings.TrimSpace(body.Chart.Error.Description))
	}
	if res.StatusCode != 200 || len(body.Chart.Result) == 0 {
		return nil, fmt.Errorf("%s: HTTP %d", ysym, res.StatusCode)
	}
	r := body.Chart.Result[0]
	out := map[Day]float64{}
	if len(r.Indicators.Quote) == 0 {
		return out, nil
	}
	closes := r.Indicators.Quote[0].Close
	fromD, toD := DayOf(from), DayOf(to)
	for i, ts := range r.Timestamp {
		if i >= len(closes) || closes[i] == nil || *closes[i] <= 0 {
			continue
		}
		d := DayOf(time.Unix(ts, 0))
		if d >= fromD && d <= toD {
			out[d] = *closes[i]
		}
	}
	return out, nil
}
