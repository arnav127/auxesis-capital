package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Visit log for admins. visits: one row per signed-in session (investors and guests, not admins).
// site_hits: anonymous daily counts for the public pages, keyed by a salted hash (no IPs stored).
func init() {
	m.Register(func(app core.App) error {
		if _, err := app.FindCollectionByNameOrId("visits"); err != nil {
			users, err := app.FindCollectionByNameOrId("users")
			if err != nil {
				return err
			}
			c := core.NewBaseCollection("visits")
			c.Fields.Add(
				&core.RelationField{Name: "user", CollectionId: users.Id, MaxSelect: 1, CascadeDelete: true},
				&core.TextField{Name: "email", Max: 200},
				&core.TextField{Name: "name", Max: 200},
				&core.TextField{Name: "role", Max: 20},
				&core.DateField{Name: "started", Required: true},
				&core.DateField{Name: "last_seen", Required: true},
				&core.NumberField{Name: "pages", OnlyInt: true},
				&core.JSONField{Name: "paths", MaxSize: 8 << 10},
				&core.TextField{Name: "device", Max: 80},
				&core.BoolField{Name: "login"},
			)
			c.AddIndex("idx_visits_user_seen", false, "user, last_seen", "")
			c.AddIndex("idx_visits_started", false, "started", "")
			if err := app.Save(c); err != nil {
				return err
			}
		}
		if _, err := app.FindCollectionByNameOrId("site_hits"); err != nil {
			c := core.NewBaseCollection("site_hits")
			c.Fields.Add(
				&core.TextField{Name: "day", Required: true, Pattern: `^\d{4}-\d{2}-\d{2}$`, Max: 10},
				&core.TextField{Name: "visitor", Required: true, Max: 32},
				&core.NumberField{Name: "views", OnlyInt: true},
				&core.BoolField{Name: "member"},
			)
			c.AddIndex("idx_site_hits_day_visitor", true, "day, visitor", "")
			if err := app.Save(c); err != nil {
				return err
			}
		}
		return nil
	}, nil)
}
