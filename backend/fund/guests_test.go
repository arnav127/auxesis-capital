package fund

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"

	_ "github.com/arnav127/auxesis-capital/backend/migrations"
)

func newApp(t *testing.T) core.App {
	t.Helper()
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.ResetBootstrapState() })
	return app
}

func TestGuestSignIn(t *testing.T) {
	s := New(newApp(t))

	if _, err := s.SignIn("prof@iima.ac.in", ""); err == nil {
		t.Fatal("unknown email signed in")
	}
	g, err := s.SaveGuest(Guest{Name: "Prof. Bala", Email: " Prof@IIMA.ac.in ", Note: "Faculty guide"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.SignIn("prof@iima.ac.in", "Bala Google Name")
	if err != nil {
		t.Fatal(err)
	}
	if u.GetString("role") != "guest" || u.GetString("name") != "Prof. Bala" {
		t.Fatalf("role %q name %q", u.GetString("role"), u.GetString("name"))
	}
	if isAdmin(u) {
		t.Fatal("guest is admin")
	}
	v, err := s.Portfolio(u, false)
	if err != nil {
		t.Fatal(err)
	}
	if v.Me != nil || len(v.Pods) != 0 {
		t.Fatal("guest sees a holding or pods")
	}

	if _, err := s.SaveGuest(Guest{Name: "x", Email: "not-an-email"}); err == nil {
		t.Fatal("bad email accepted")
	}

	if err := s.DeleteGuest(g.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.app.FindAuthRecordByEmail("users", "prof@iima.ac.in"); err == nil {
		t.Fatal("guest account survived removal")
	}
	if _, err := s.SignIn("prof@iima.ac.in", ""); err == nil {
		t.Fatal("removed guest signed in")
	}
}
