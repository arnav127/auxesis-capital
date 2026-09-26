package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Adds the NAV at which units were allotted to each investor transaction, and fills in the
// 2026–27 team (only if the team is still empty, so later edits in the dashboard are kept).
func init() {
	m.Register(func(app core.App) error {
		txns, err := app.FindCollectionByNameOrId("investor_txns")
		if err != nil {
			return err
		}
		// NAV per unit the units were allotted at (1000 at launch). With units left 0, units = amount / nav.
		txns.Fields.Add(&core.NumberField{Name: "nav"})
		if err := app.Save(txns); err != nil {
			return err
		}

		if n, _ := app.CountRecords("team"); n > 0 {
			return nil
		}
		team, err := app.FindCollectionByNameOrId("team")
		if err != nil {
			return err
		}
		type member struct{ name, role, group, org string }
		people := []member{
			{"Arnav Dixit", "Coordinator", "leadership", "Beta, The Finance & Investments Club"},
			{"Bhavya Jain", "Fund Manager", "leadership", "Auxesis Capital"},
			{"Aryansh Kabra", "Head, Investments Cell", "leadership", "Beta"},
		}
		for _, n := range []string{"Aditya Patwa", "Gurasheesh Singh", "Heth Doshi", "Krishiv Agarwal", "Raghav Bagri", "Rajit Agarwal"} {
			people = append(people, member{n, "Senior Analyst", "pgp2", ""})
		}
		for _, n := range []string{"Megha Goenka", "Satwik Murarka", "Vatsal Bhura", "Madhuwanthan Madhav Kumar"} {
			people = append(people, member{n, "Analyst", "pgp1", ""})
		}
		for i, p := range people {
			r := core.NewRecord(team)
			r.Load(map[string]any{"name": p.name, "role": p.role, "group": p.group, "org": p.org, "order": i})
			if err := app.Save(r); err != nil {
				return err
			}
		}
		return nil
	}, nil)
}
