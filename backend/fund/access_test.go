package fund

import "testing"

// The platform signs in any @iima.ac.in account; Access narrows that to the fund's own people.
func TestAccess(t *testing.T) {
	s := New(newApp(t))

	u, err := s.Access("p25stranger@iima.ac.in", "Stranger", false)
	if err != nil || u != nil {
		t.Fatalf("unregistered: got %v, %v; want nil, nil (signed out in this app)", u, err)
	}

	if _, err := s.SaveGuest(Guest{Name: "Prof. Bala", Email: "prof@iima.ac.in"}); err != nil {
		t.Fatal(err)
	}
	u, err = s.Access("prof@iima.ac.in", "Bala Google Name", false)
	if err != nil || u == nil || u.GetString("role") != "guest" {
		t.Fatalf("guest: got %v, %v", u, err)
	}

	// Fund admins come from the platform role, not from a list in this app.
	u, err = s.Access("boss@iima.ac.in", "Boss", true)
	if err != nil || u == nil || u.GetString("role") != "admin" {
		t.Fatalf("admin: got %v, %v", u, err)
	}
	// Role taken away on the platform: on the next request, a guest is a guest again, and
	// someone with no other standing is signed out here.
	u, err = s.Access("prof@iima.ac.in", "Prof", true)
	if err != nil || u.GetString("role") != "admin" {
		t.Fatalf("guest made admin: %v %v", u, err)
	}
	u, err = s.Access("prof@iima.ac.in", "Prof", false)
	if err != nil || u.GetString("role") != "guest" {
		t.Fatalf("admin role removed: %v %v", u, err)
	}
	if u, err = s.Access("boss@iima.ac.in", "Boss", false); err != nil || u != nil {
		t.Fatalf("ex-admin: got %v, %v", u, err)
	}
}
