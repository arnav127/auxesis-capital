package fund

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// A visit is one signed-in session: page loads within visitGap of each other join it.
const visitGap = 30 * time.Minute

var visitMu sync.Mutex

// CleanPath keeps an app path worth logging ("/publications/q2-letter"), or returns "".
func CleanPath(p string) string {
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	p = strings.TrimSpace(p)
	if !strings.HasPrefix(p, "/") || len(p) > 120 || p == "/login" || p == "/auth/callback" {
		return ""
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}

// Device summarises a user agent as "Phone · Chrome".
func Device(ua string) string {
	u := strings.ToLower(ua)
	kind := "Desktop"
	switch {
	case strings.Contains(u, "ipad") || strings.Contains(u, "tablet"):
		kind = "Tablet"
	case strings.Contains(u, "mobi") || strings.Contains(u, "iphone") || strings.Contains(u, "android"):
		kind = "Phone"
	}
	browser := "Browser"
	for _, b := range []struct{ key, name string }{
		{"edg/", "Edge"}, {"opr/", "Opera"}, {"samsungbrowser", "Samsung Internet"}, {"crios", "Chrome"}, {"fxios", "Firefox"},
		{"firefox", "Firefox"}, {"chrome", "Chrome"}, {"safari", "Safari"},
	} {
		if strings.Contains(u, b.key) {
			browser = b.name
			break
		}
	}
	return kind + " · " + browser
}

func (s *Service) visitSalt() string {
	s.saltOnce.Do(func() {
		p := filepath.Join(s.app.DataDir(), "visit_salt")
		if b, err := os.ReadFile(p); err == nil && len(b) >= 16 {
			s.salt = string(b)
			return
		}
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		s.salt = hex.EncodeToString(b)
		_ = os.WriteFile(p, []byte(s.salt), 0o600)
	})
	return s.salt
}

// RecordVisit logs a page view. Signed-in investors and guests extend (or start) their visit;
// everyone except admins counts towards the anonymous daily totals.
func (s *Service) RecordVisit(u *core.Record, path, ip, ua string, login bool, now time.Time) error {
	if u != nil && isAdmin(u) {
		return nil
	}
	path = CleanPath(path)
	if path == "" && !login {
		return nil
	}
	visitMu.Lock()
	defer visitMu.Unlock()

	day := now.In(IST).Format("2006-01-02")
	if path != "" {
		who := ip + "|" + ua
		if u != nil {
			who = "user:" + u.Id
		}
		sum := sha256.Sum256([]byte(s.visitSalt() + "|" + day + "|" + who))
		visitor := hex.EncodeToString(sum[:])[:32]
		hit, _ := s.app.FindFirstRecordByFilter("site_hits", "day = {:d} && visitor = {:v}", map[string]any{"d": day, "v": visitor})
		if hit == nil {
			c, err := s.app.FindCollectionByNameOrId("site_hits")
			if err != nil {
				return err
			}
			hit = core.NewRecord(c)
			hit.Load(map[string]any{"day": day, "visitor": visitor})
		}
		hit.Set("views", hit.GetInt("views")+1)
		if u != nil {
			hit.Set("member", true)
		}
		if err := s.app.Save(hit); err != nil {
			return err
		}
	}
	if u == nil {
		return nil
	}

	nowDT, _ := types.ParseDateTime(now)
	var v *core.Record
	if !login {
		cut, _ := types.ParseDateTime(now.Add(-visitGap))
		rows, _ := s.app.FindRecordsByFilter("visits", "user = {:u} && last_seen >= {:t}", "-last_seen", 1, 0, map[string]any{"u": u.Id, "t": cut.String()})
		if len(rows) > 0 {
			v = rows[0]
		}
	}
	if v == nil {
		c, err := s.app.FindCollectionByNameOrId("visits")
		if err != nil {
			return err
		}
		v = core.NewRecord(c)
		v.Load(map[string]any{"user": u.Id, "started": nowDT, "login": login, "paths": []string{}})
	}
	name := u.GetString("name")
	if inv := s.InvestorFor(u.GetString("email")); inv != nil {
		name = inv.GetString("name") // as the admins know them
	}
	v.Load(map[string]any{"email": u.GetString("email"), "name": name, "role": u.GetString("role"), "last_seen": nowDT})
	if d := Device(ua); ua != "" && (v.GetString("device") == "" || login) {
		v.Set("device", d)
	}
	if path != "" {
		v.Set("pages", v.GetInt("pages")+1)
		var paths []string
		_ = v.UnmarshalJSONField("paths", &paths)
		if !slices.Contains(paths, path) && len(paths) < 40 {
			v.Set("paths", append(paths, path))
		}
	}
	return s.app.Save(v)
}

// PruneHits drops anonymous daily counts older than a year.
func (s *Service) PruneHits(now time.Time) {
	cut := now.In(IST).AddDate(-1, 0, 0).Format("2006-01-02")
	rows, err := s.app.FindRecordsByFilter("site_hits", "day < {:d}", "", 0, 0, map[string]any{"d": cut})
	if err != nil {
		return
	}
	for _, r := range rows {
		_ = s.app.Delete(r)
	}
}

// ---------- the admin view ----------

type VisitRow struct {
	Name   string   `json:"name"`
	Email  string   `json:"email"`
	Role   string   `json:"role"`
	At     string   `json:"at"`
	Last   string   `json:"last"`
	Pages  int      `json:"pages"`
	Paths  []string `json:"paths"`
	Device string   `json:"device"`
	Login  bool     `json:"login"`
}

type PersonVisits struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Visits int    `json:"visits"`
	Pages  int    `json:"pages"`
	First  string `json:"first"`
	Last   string `json:"last"`
	Device string `json:"device"`
}

type DayVisits struct {
	D      string `json:"d"`
	Visits int    `json:"visits"`
	People int    `json:"people"`
	Public int    `json:"public"`
}

type Readers struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Readers int    `json:"readers"`
}

type VisitStats struct {
	Investors struct {
		Total    int `json:"total"`
		SignedIn int `json:"signedIn"`
		Active7  int `json:"active7"`
		Active30 int `json:"active30"`
	} `json:"investors"`
	Visits7  int `json:"visits7"`
	Visits30 int `json:"visits30"`
	Public   struct {
		Today   int `json:"today"`
		Week    int `json:"week"`
		Month   int `json:"month"`
		Views30 int `json:"views30"`
	} `json:"public"`
	Daily  []DayVisits    `json:"daily"`
	People []PersonVisits `json:"people"`
	Never  []PersonVisits `json:"never"`
	Recent []VisitRow     `json:"recent"`
	Reads  []Readers      `json:"reads"`
}

// VisitStats summarises the visit log for the admin page, in IST days.
func (s *Service) VisitStats(now time.Time) (*VisitStats, error) {
	out := &VisitStats{Daily: []DayVisits{}, People: []PersonVisits{}, Never: []PersonVisits{}, Recent: []VisitRow{}, Reads: []Readers{}}
	today := now.In(IST)
	dayOf := func(t time.Time) string { return t.In(IST).Format("2006-01-02") }
	days := map[string]*DayVisits{}
	for i := 29; i >= 0; i-- {
		d := today.AddDate(0, 0, -i).Format("2006-01-02")
		out.Daily = append(out.Daily, DayVisits{D: d})
		days[d] = &out.Daily[len(out.Daily)-1]
	}
	since := func(n int) string { return today.AddDate(0, 0, -n+1).Format("2006-01-02") }

	visits, err := s.app.FindRecordsByFilter("visits", "", "-started", 0, 0)
	if err != nil {
		return nil, err
	}
	people := map[string]*PersonVisits{}
	dayPeople := map[string]map[string]bool{}
	readers := map[string]map[string]bool{}
	a7, a30 := map[string]bool{}, map[string]bool{}
	for i, v := range visits {
		email := v.GetString("email")
		started, last := v.GetDateTime("started").Time(), v.GetDateTime("last_seen").Time()
		var paths []string
		_ = v.UnmarshalJSONField("paths", &paths)
		if i < 60 {
			out.Recent = append(out.Recent, VisitRow{v.GetString("name"), email, v.GetString("role"), started.Format(time.RFC3339), last.Format(time.RFC3339), v.GetInt("pages"), paths, v.GetString("device"), v.GetBool("login")})
		}
		p := people[email]
		if p == nil {
			p = &PersonVisits{Name: v.GetString("name"), Email: email, Role: v.GetString("role"), Last: last.Format(time.RFC3339), Device: v.GetString("device")}
			people[email] = p
		}
		p.Visits++
		p.Pages += v.GetInt("pages")
		p.First = started.Format(time.RFC3339)
		if d := dayOf(started); days[d] != nil {
			days[d].Visits++
			if dayPeople[d] == nil {
				dayPeople[d] = map[string]bool{}
			}
			dayPeople[d][email] = true
		}
		ds := dayOf(last)
		if ds >= since(7) {
			out.Visits7++
			a7[email] = true
		}
		if ds >= since(30) {
			out.Visits30++
			a30[email] = true
		}
		for _, path := range paths {
			if slug, ok := strings.CutPrefix(path, "/publications/"); ok && slug != "" {
				if readers[slug] == nil {
					readers[slug] = map[string]bool{}
				}
				readers[slug][email] = true
			}
		}
	}
	for d, set := range dayPeople {
		days[d].People = len(set)
	}

	// Investors: who has signed in (a users record exists) and who is active.
	invs, err := s.app.FindRecordsByFilter("investors", "", "name", 0, 0)
	if err != nil {
		return nil, err
	}
	out.Investors.Total = len(invs)
	for _, inv := range invs {
		email := strings.ToLower(inv.GetString("email"))
		if email == "" {
			continue
		}
		_, uErr := s.app.FindAuthRecordByEmail("users", email)
		if uErr == nil || people[email] != nil {
			out.Investors.SignedIn++
		} else {
			out.Never = append(out.Never, PersonVisits{Name: inv.GetString("name"), Email: email, Role: "investor"})
		}
		if a7[email] {
			out.Investors.Active7++
		}
		if a30[email] {
			out.Investors.Active30++
		}
	}
	for _, p := range people {
		out.People = append(out.People, *p)
	}
	sort.Slice(out.People, func(i, j int) bool { return out.People[i].Last > out.People[j].Last })

	hits, err := s.app.FindRecordsByFilter("site_hits", "day >= {:d}", "", 0, 0, map[string]any{"d": since(30)})
	if err != nil {
		return nil, err
	}
	for _, h := range hits {
		d := h.GetString("day")
		out.Public.Month++
		out.Public.Views30 += h.GetInt("views")
		if d >= since(7) {
			out.Public.Week++
		}
		if d == since(1) {
			out.Public.Today++
		}
		if days[d] != nil {
			days[d].Public++
		}
	}

	for slug, set := range readers {
		title := slug
		if r, err := s.app.FindFirstRecordByFilter("reports", "slug = {:s}", map[string]any{"s": slug}); err == nil {
			title = r.GetString("title")
		}
		out.Reads = append(out.Reads, Readers{slug, title, len(set)})
	}
	sort.Slice(out.Reads, func(i, j int) bool {
		if out.Reads[i].Readers != out.Reads[j].Readers {
			return out.Reads[i].Readers > out.Reads[j].Readers
		}
		return out.Reads[i].Title < out.Reads[j].Title
	})
	return out, nil
}
