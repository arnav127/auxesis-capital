package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// 2026–27 desk changes: Nikhil Singh Bohra has left, and Aditya Patwa joins the PGP2 senior analysts.
func init() {
	m.Register(func(app core.App) error {
		gone, err := app.FindRecordsByFilter("team", "name ~ 'Nikhil'", "", 0, 0)
		if err != nil {
			return err
		}
		for _, r := range gone {
			if err := app.Delete(r); err != nil {
				return err
			}
		}

		if found, _ := app.FindRecordsByFilter("team", "name = 'Aditya Patwa'", "", 1, 0); len(found) > 0 {
			return nil
		}
		team, err := app.FindCollectionByNameOrId("team")
		if err != nil {
			return err
		}
		// Share the first senior analyst's order, so he sorts alphabetically at the top of PGP2.
		order := 0
		if first, err := app.FindRecordsByFilter("team", "group = 'pgp2'", "order", 1, 0); err == nil && len(first) > 0 {
			order = first[0].GetInt("order")
		}
		r := core.NewRecord(team)
		r.Load(map[string]any{"name": "Aditya Patwa", "role": "Senior Analyst", "group": "pgp2", "order": order})
		return app.Save(r)
	}, nil)
}
