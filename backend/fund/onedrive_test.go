package fund

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownloadCandidates(t *testing.T) {
	got := DownloadCandidates("https://iima1-my.sharepoint.com/:x:/g/personal/p25aryanshk_iima_ac_in/IQDy0WLMoCDu?e=5Ys71X")
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
	if got[0] != "https://iima1-my.sharepoint.com/personal/p25aryanshk_iima_ac_in/_layouts/15/download.aspx?share=IQDy0WLMoCDu" {
		t.Errorf("direct: %s", got[0])
	}
	if !strings.Contains(got[1], "download=1") || !strings.Contains(got[1], "e=5Ys71X") {
		t.Errorf("link: %s", got[1])
	}
	if g := DownloadCandidates("https://example.com/f.xlsx"); len(g) != 1 || g[0] != "https://example.com/f.xlsx" {
		t.Errorf("plain: %v", g)
	}
}

// Simulates SharePoint: download.aspx wants a guest cookie, which the sharing link sets on its way
// through guestaccess.aspx.
func sharepointSim(t *testing.T, directWorks bool) *httptest.Server {
	xlsx := buildTracker(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/:x:/g/personal/u/SHARE", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("download") != "1" {
			w.Write([]byte("<html><title>Excel</title></html>"))
			return
		}
		http.Redirect(w, r, "/personal/u/_layouts/15/guestaccess.aspx?share=SHARE", http.StatusFound)
	})
	mux.HandleFunc("/personal/u/_layouts/15/guestaccess.aspx", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "FedAuth", Value: "guest", Path: "/"})
		http.Redirect(w, r, "/personal/u/_layouts/15/download.aspx?share=SHARE", http.StatusFound)
	})
	mux.HandleFunc("/personal/u/_layouts/15/download.aspx", func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("FedAuth"); err != nil && !directWorks {
			http.Redirect(w, r, "/_forms/default.aspx?ReturnUrl=x", http.StatusFound)
			return
		}
		w.Write(xlsx)
	})
	mux.HandleFunc("/_forms/default.aspx", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><title>Sign in to your account</title></html>"))
	})
	return httptest.NewServer(mux)
}

func TestFetchShared(t *testing.T) {
	for _, direct := range []bool{true, false} {
		srv := sharepointSim(t, direct)
		data, err := fetchShared(context.Background(), srv.Client(), srv.URL+"/:x:/g/personal/u/SHARE?e=abc")
		srv.Close()
		if err != nil || !strings.HasPrefix(string(data), "PK") {
			t.Fatalf("direct=%v: %v", direct, err)
		}
	}
	// Only a sign-in page: a clear error naming what came back.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/_forms/") {
			http.Redirect(w, r, "/_forms/default.aspx", http.StatusFound)
			return
		}
		w.Write([]byte("<html><title>Sign in to your account</title></html>"))
	}))
	defer srv.Close()
	_, err := fetchShared(context.Background(), srv.Client(), srv.URL+"/:x:/g/personal/u/SHARE?e=abc")
	if !errors.Is(err, ErrNeedsSignIn) || !strings.Contains(err.Error(), "sign-in page") {
		t.Errorf("%v", err)
	}
}
