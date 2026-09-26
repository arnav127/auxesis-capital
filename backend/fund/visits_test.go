package fund

import (
	"testing"
	"time"
)

func TestRecordVisitAndStats(t *testing.T) {
	s := New(newApp(t))
	if _, err := s.SaveInvestor(InvestorInput{Name: "Asha Rao", Email: "asha@iima.ac.in", Amount: 20000, NAV: 1000}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveInvestor(InvestorInput{Name: "Ravi K", Email: "ravi@iima.ac.in", Amount: 10000, NAV: 1000}); err != nil {
		t.Fatal(err)
	}
	u, err := s.SignIn("asha@iima.ac.in", "")
	if err != nil {
		t.Fatal(err)
	}
	const iphone = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Version/18.0 Mobile/15E148 Safari/604.1"
	t0 := time.Date(2026, 9, 26, 10, 0, 0, 0, IST)

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.RecordVisit(u, "", "1.2.3.4", iphone, true, t0))                                                // sign-in starts a visit
	must(s.RecordVisit(u, "/portfolio", "1.2.3.4", iphone, false, t0.Add(time.Minute)))                    // joins it
	must(s.RecordVisit(u, "/publications/q2-letter?x=1", "1.2.3.4", iphone, false, t0.Add(9*time.Minute))) // joins it
	must(s.RecordVisit(u, "/portfolio", "1.2.3.4", iphone, false, t0.Add(3*time.Hour)))                    // a new visit
	must(s.RecordVisit(nil, "/", "9.9.9.9", "Mozilla/5.0 (Windows NT 10.0) Chrome/130", false, t0))        // public
	must(s.RecordVisit(nil, "/team", "9.9.9.9", "Mozilla/5.0 (Windows NT 10.0) Chrome/130", false, t0.Add(time.Minute)))
	must(s.RecordVisit(nil, "/login", "8.8.8.8", "x", false, t0)) // not logged

	st, err := s.VisitStats(t0.Add(4 * time.Hour))
	must(err)
	if len(st.Recent) != 2 {
		t.Fatalf("visits %d, want 2", len(st.Recent))
	}
	first := st.Recent[1]
	if !first.Login || first.Pages != 2 || first.Device != "Phone · Safari" || len(first.Paths) != 2 || first.Paths[1] != "/publications/q2-letter" {
		t.Fatalf("first visit %+v", first)
	}
	if st.Investors.Total != 2 || st.Investors.SignedIn != 1 || st.Investors.Active7 != 1 || len(st.Never) != 1 || st.Never[0].Email != "ravi@iima.ac.in" {
		t.Fatalf("investors %+v never %+v", st.Investors, st.Never)
	}
	if len(st.People) != 1 || st.People[0].Visits != 2 || st.People[0].Pages != 3 {
		t.Fatalf("people %+v", st.People)
	}
	// Two unique visitors today: Asha and the anonymous one (two pages).
	if st.Public.Today != 2 || st.Public.Views30 != 5 {
		t.Fatalf("public %+v", st.Public)
	}
	if last := st.Daily[len(st.Daily)-1]; last.D != "2026-09-26" || last.Visits != 2 || last.People != 1 || last.Public != 2 {
		t.Fatalf("today %+v", last)
	}
	if len(st.Reads) != 1 || st.Reads[0].Readers != 1 {
		t.Fatalf("reads %+v", st.Reads)
	}
}

func TestAdminsNotLogged(t *testing.T) {
	s := New(newApp(t))
	t.Setenv("ADMIN_EMAILS", "boss@iima.ac.in")
	u, err := s.SignIn("boss@iima.ac.in", "Boss")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordVisit(u, "/portfolio", "1.1.1.1", "ua", false, time.Now()); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.app.CountRecords("site_hits"); n != 0 {
		t.Fatal("admin page view counted")
	}
}

func TestDevice(t *testing.T) {
	for ua, want := range map[string]string{
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 Chrome/130.0 Mobile Safari/537.36":  "Phone · Chrome",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/605.1.15 Version/17.0 Safari/605.1.15": "Desktop · Safari",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/130.0 Safari/537.36 Edg/130.0":                 "Desktop · Edge",
	} {
		if got := Device(ua); got != want {
			t.Errorf("%s: %s, want %s", ua, got, want)
		}
	}
}
