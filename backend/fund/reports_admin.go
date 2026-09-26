package fund

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// ReportDraft is a report as the site's editor sees it (published or not).
type ReportDraft struct {
	ID        string   `json:"id"`
	Slug      string   `json:"slug"`
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	Category  string   `json:"category"`
	Date      string   `json:"date"`
	Access    string   `json:"access"`
	Author    string   `json:"author"`
	Dek       string   `json:"dek"`
	Summary   string   `json:"summary"`
	Kicker    string   `json:"kicker"`
	KickerSub string   `json:"kickerSub"`
	Facts     []Fact   `json:"facts"`
	Body      string   `json:"body"`
	Pages     string   `json:"pages"`
	Published bool     `json:"published"`
	PDF       string   `json:"pdf"`
	Images    []string `json:"images"`
	Updated   string   `json:"updated"`
}

type Fact struct {
	K  string `json:"k"`
	V  string `json:"v"`
	Up bool   `json:"up,omitempty"`
}

func draftOf(r *core.Record) ReportDraft {
	d := ReportDraft{ID: r.Id, Slug: r.GetString("slug"), Title: r.GetString("title"), Type: r.GetString("type"), Category: r.GetString("category"),
		Date: r.GetString("date"), Access: r.GetString("access"), Author: r.GetString("author"), Dek: r.GetString("dek"), Summary: r.GetString("summary"),
		Kicker: r.GetString("kicker"), KickerSub: r.GetString("kickerSub"), Body: r.GetString("body"), Pages: r.GetString("pages"),
		Published: r.GetBool("published"), PDF: r.GetString("pdf"), Images: r.GetStringSlice("images"), Facts: []Fact{},
		Updated: r.GetDateTime("updated").Time().Format(time.RFC3339)}
	if raw, err := json.Marshal(r.Get("facts")); err == nil {
		_ = json.Unmarshal(raw, &d.Facts)
	}
	if d.Facts == nil {
		d.Facts = []Fact{}
	}
	if d.Images == nil {
		d.Images = []string{}
	}
	return d
}

// AllReports lists every report, drafts included, newest first (without bodies).
func (s *Service) AllReports() ([]ReportDraft, error) {
	rows, err := s.app.FindRecordsByFilter("reports", "", "-date,-created", 0, 0)
	if err != nil {
		return nil, err
	}
	out := []ReportDraft{}
	for _, r := range rows {
		d := draftOf(r)
		d.Body = ""
		out = append(out, d)
	}
	return out, nil
}

func (s *Service) ReportDraftByID(id string) (*ReportDraft, error) {
	r, err := s.app.FindRecordById("reports", id)
	if err != nil {
		return nil, fmt.Errorf("that publication no longer exists")
	}
	d := draftOf(r)
	return &d, nil
}

var slugClean = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a title into a URL slug: "Letter to Investors, Q1 FY27" → "letter-to-investors-q1-fy27".
func Slugify(s string) string {
	s = strings.Trim(slugClean.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(s) > 80 {
		s = strings.Trim(s[:80], "-")
	}
	return s
}

// SaveReport creates or updates a report. pdf (optional) replaces the PDF; removePDF drops it.
func (s *Service) SaveReport(d ReportDraft, pdf *filesystem.File, removePDF bool) (*ReportDraft, error) {
	d.Title = strings.TrimSpace(d.Title)
	if d.Title == "" {
		return nil, fmt.Errorf("a title is needed")
	}
	if d.Slug = Slugify(d.Slug); d.Slug == "" {
		d.Slug = Slugify(d.Title)
	}
	if strings.TrimSpace(d.Type) == "" {
		return nil, fmt.Errorf("choose the kind of publication (e.g. Quarterly Letter)")
	}
	if !slices.Contains([]string{"Letters", "Factsheets", "Research", "Macro"}, d.Category) {
		return nil, fmt.Errorf("choose a category")
	}
	if d.Access != "public" && d.Access != "investors" {
		return nil, fmt.Errorf("choose who can read it")
	}
	if d.Date == "" {
		d.Date = today()
	}
	if _, err := time.Parse("2006-01-02", d.Date); err != nil {
		return nil, fmt.Errorf("the date should look like 2026-07-18")
	}
	facts := []Fact{}
	for _, f := range d.Facts {
		if strings.TrimSpace(f.K) != "" || strings.TrimSpace(f.V) != "" {
			facts = append(facts, Fact{K: strings.TrimSpace(f.K), V: strings.TrimSpace(f.V), Up: f.Up})
		}
	}

	var r *core.Record
	if d.ID != "" {
		rec, err := s.app.FindRecordById("reports", d.ID)
		if err != nil {
			return nil, fmt.Errorf("that publication no longer exists")
		}
		r = rec
	} else {
		c, err := s.app.FindCollectionByNameOrId("reports")
		if err != nil {
			return nil, err
		}
		r = core.NewRecord(c)
	}
	if other, _ := s.app.FindFirstRecordByData("reports", "slug", d.Slug); other != nil && other.Id != r.Id {
		return nil, fmt.Errorf("another publication already uses the address /%s; change the URL slug", d.Slug)
	}
	r.Load(map[string]any{"slug": d.Slug, "title": d.Title, "type": strings.TrimSpace(d.Type), "category": d.Category, "date": d.Date,
		"access": d.Access, "author": strings.TrimSpace(d.Author), "dek": strings.TrimSpace(d.Dek), "summary": strings.TrimSpace(d.Summary),
		"kicker": strings.TrimSpace(d.Kicker), "kickerSub": strings.TrimSpace(d.KickerSub), "facts": facts, "body": d.Body,
		"pages": strings.TrimSpace(d.Pages), "published": d.Published})
	if pdf != nil {
		r.Set("pdf", pdf)
	} else if removePDF {
		r.Set("pdf", nil)
	}
	if err := s.app.Save(r); err != nil {
		return nil, err
	}
	out := draftOf(r)
	return &out, nil
}

// AddReportImages uploads images to a report and returns the stored file names.
func (s *Service) AddReportImages(id string, files []*filesystem.File) ([]string, error) {
	r, err := s.app.FindRecordById("reports", id)
	if err != nil {
		return nil, fmt.Errorf("save the publication first, then add images")
	}
	before := r.GetStringSlice("images")
	r.Set("images+", files)
	if err := s.app.Save(r); err != nil {
		return nil, err
	}
	var added []string
	for _, n := range r.GetStringSlice("images") {
		if !slices.Contains(before, n) {
			added = append(added, n)
		}
	}
	return added, nil
}

func (s *Service) DeleteReport(id string) error {
	r, err := s.app.FindRecordById("reports", id)
	if err != nil {
		return fmt.Errorf("that publication no longer exists")
	}
	return s.app.Delete(r)
}
