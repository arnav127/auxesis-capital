package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Guests (e.g. the faculty guide) can sign in and read the portfolio and investor-only publications
// without holding units. They never see pods or the admin pages.
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		if f, ok := users.Fields.GetByName("role").(*core.SelectField); ok {
			f.Values = []string{"investor", "admin", "guest"}
		}
		if err := app.Save(users); err != nil {
			return err
		}

		if _, err := app.FindCollectionByNameOrId("guests"); err == nil {
			return nil
		}
		c := core.NewBaseCollection("guests")
		c.Fields.Add(
			&core.EmailField{Name: "email", Required: true},
			&core.TextField{Name: "name", Required: true, Max: 200},
			&core.TextField{Name: "note", Max: 200},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		c.AddIndex("idx_guests_email", true, "email", "")
		return app.Save(c)
	}, nil)
}
