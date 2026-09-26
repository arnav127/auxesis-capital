package fund

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestYahooCloses(t *testing.T) {
	// Two sessions (09:15 IST opens), one missing close.
	body := `{"chart":{"result":[{"timestamp":[1783914300,1784000700,1784087100],"indicators":{"quote":[{"close":[101.5,null,103.25]}]}}],"error":null}}`
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.EscapedPath()
		if !strings.Contains(r.UserAgent(), "Mozilla") {
			w.WriteHeader(403)
			return
		}
		w.Write([]byte(body))
	}))
	defer srv.Close()
	y := &Yahoo{BaseURL: srv.URL}
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, IST)
	got, err := y.Closes(context.Background(), "M&M.NS", from, from.AddDate(0, 1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/M&M.NS") && !strings.HasSuffix(path, "/M%26M.NS") {
		t.Errorf("path %s", path)
	}
	if len(got) != 2 || got["2026-07-13"] != 101.5 || got["2026-07-15"] != 103.25 {
		t.Errorf("closes %v", got)
	}
	if YahooSymbol(Nifty50, "") != "^NSEI" || YahooSymbol("TITAN", "") != "TITAN.NS" || YahooSymbol("X", "X.BO") != "X.BO" {
		t.Error("symbol mapping")
	}
}

func TestYahooError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`))
	}))
	defer srv.Close()
	_, err := (&Yahoo{BaseURL: srv.URL}).Closes(context.Background(), "GONE.NS", time.Now().AddDate(0, -1, 0), time.Now())
	if err == nil || !strings.Contains(err.Error(), "delisted") {
		t.Errorf("err %v", err)
	}
}
