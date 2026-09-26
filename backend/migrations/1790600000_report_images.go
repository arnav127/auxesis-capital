package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Images uploaded from the site's report editor. Served through /api/aux/reports/{slug}/img/{name}
// with the same access check as the report, so investors-only images stay private.
func init() {
	m.Register(func(app core.App) error {
		reports, err := app.FindCollectionByNameOrId("reports")
		if err != nil {
			return err
		}
		reports.Fields.Add(&core.FileField{Name: "images", MaxSelect: 40, MaxSize: 10 << 20, Protected: true,
			MimeTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif", "image/svg+xml"}})
		return app.Save(reports)
	}, nil)
}
