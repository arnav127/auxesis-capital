package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// The first migration seeded a placeholder motto from the design mockups. Clear it so the site
// shows a motto only once the fund sets one (fund_settings → motto).
func init() {
	m.Register(func(app core.App) error {
		rows, err := app.FindRecordsByFilter("fund_settings", "motto = 'Per ardua ad alta'", "", 0, 0)
		if err != nil {
			return err
		}
		for _, r := range rows {
			r.Set("motto", "")
			if err := app.Save(r); err != nil {
				return err
			}
		}
		return nil
	}, nil)
}
