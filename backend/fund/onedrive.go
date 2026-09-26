package fund

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Downloading the tracker from a OneDrive / SharePoint "Anyone with the link" share.
//
// A sharing link such as
//
//	https://iima1-my.sharepoint.com/:x:/g/personal/<user>/<shareId>?e=abc
//
// opens Excel Online in a browser. SharePoint serves the file itself from
//
//	https://iima1-my.sharepoint.com/personal/<user>/_layouts/15/download.aspx?share=<shareId>
//
// or from the sharing link with download=1, which first redirects through a guest-access page
// that sets a cookie. We try the direct address first, then the link with a cookie jar.

var sharePath = regexp.MustCompile(`^/:[a-z]:/[a-z]/personal/([^/]+)/([^/?]+)`)

// DownloadCandidates lists the addresses to try, best first.
func DownloadCandidates(link string) []string {
	link = strings.TrimSpace(link)
	u, err := url.Parse(link)
	if err != nil || u.Host == "" {
		return []string{link}
	}
	var out []string
	if m := sharePath.FindStringSubmatch(u.Path); m != nil {
		d := url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/personal/" + m[1] + "/_layouts/15/download.aspx", RawQuery: "share=" + url.QueryEscape(m[2])}
		out = append(out, d.String())
	}
	h := strings.ToLower(u.Host)
	if len(out) > 0 || strings.HasSuffix(h, "sharepoint.com") || strings.HasSuffix(h, "1drv.ms") || strings.HasSuffix(h, "onedrive.live.com") {
		q := u.Query()
		q.Set("download", "1")
		v := *u
		v.RawQuery = q.Encode()
		out = append(out, v.String())
	} else {
		out = append(out, link)
	}
	return out
}

// DownloadURL is the first address we try for a sharing link.
func DownloadURL(link string) string { return DownloadCandidates(link)[0] }

var htmlTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// fetchShared downloads an .xlsx from a sharing link, trying each candidate address with its own
// cookie jar. The error says what SharePoint returned for each attempt.
func fetchShared(ctx context.Context, base *http.Client, link string) ([]byte, error) {
	var notes []string
	signIn := false
	for i, addr := range DownloadCandidates(link) {
		jar, _ := cookiejar.New(nil)
		c := &http.Client{Jar: jar, Timeout: 60 * time.Second}
		if base != nil {
			c.Transport = base.Transport
			if base.Timeout > 0 {
				c.Timeout = base.Timeout
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
		if err != nil {
			return nil, err
		}
		// SharePoint serves the file to browsers; look like one.
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126 Safari/537.36")
		req.Header.Set("Accept", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/octet-stream,*/*")
		res, err := c.Do(req)
		if err != nil {
			notes = append(notes, fmt.Sprintf("%d) couldn't connect: %v", i+1, err))
			continue
		}
		data, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
		res.Body.Close()
		if err != nil {
			notes = append(notes, fmt.Sprintf("%d) the download was cut off: %v", i+1, err))
			continue
		}
		if res.StatusCode == 200 && bytes.HasPrefix(data, []byte("PK")) {
			return data, nil
		}
		final := res.Request.URL
		what := fmt.Sprintf("HTTP %d from %s%s", res.StatusCode, final.Host, final.Path)
		if strings.Contains(final.Host, "login.microsoftonline.com") || strings.Contains(final.Path, "/_forms/") || strings.Contains(strings.ToLower(final.Path), "signin") {
			what = "a Microsoft sign-in page (" + final.Host + ")"
			signIn = true
		} else if m := htmlTitle.FindSubmatch(data); m != nil {
			what += fmt.Sprintf(", a web page titled %q", strings.TrimSpace(string(m[1])))
		} else if ct := res.Header.Get("Content-Type"); ct != "" {
			what += ", " + ct
		}
		if res.StatusCode == 401 || res.StatusCode == 403 {
			signIn = true
		}
		notes = append(notes, fmt.Sprintf("%d) %s", i+1, what))
	}
	msg := "SharePoint didn't hand over the file. " + strings.Join(notes, "; ") + "."
	if signIn {
		return nil, fmt.Errorf("%w. %s", ErrNeedsSignIn, msg)
	}
	return nil, errors.New(msg + " Check that EXCEL_URL is the file's current sharing link, or upload the .xlsx on the Admin page")
}
