package fund

import "testing"

// The platform signs in any @iima.ac.in account; Access narrows that to the fund's own people.
func TestAccess(t *testing.T) {
	s := New(newApp(t))

	u, err := s.Access("p25stranger@iima.ac.in", "Stranger")
	if err != nil || u != nil {
		t.Fatalf("unregistered: got %v, %v; want nil, nil (signed out in this app)", u, err)
	}

	if _, err := s.SaveGuest(Guest{Name: "Prof. Bala", Email: "prof@iima.ac.in"}); err != nil {
		t.Fatal(err)
	}
	u, err = s.Access("prof@iima.ac.in", "Bala Google Name")
	if err != nil || u == nil || u.GetString("role") != "guest" {
		t.Fatalf("guest: got %v, %v", u, err)
	}

	t.Setenv("ADMIN_EMAILS", "boss@iima.ac.in")
	u, err = s.Access("boss@iima.ac.in", "Boss")
	if err != nil || u == nil || u.GetString("role") != "admin" {
		t.Fatalf("admin: got %v, %v", u, err)
	}
}
