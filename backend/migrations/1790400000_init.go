package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Schema for the Auxesis Capital site.
//
// Sign-in is Google OAuth2 only, for people on the investor list (or ADMIN_EMAILS).
// The site reads everything through the custom /api/aux/* routes, so collection API rules
// stay locked (superusers only) except the team, which is public. Superusers edit
// everything in the PocketBase dashboard at /_/.
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		users.ListRule = types.Pointer("id = @request.auth.id")
		users.ViewRule = types.Pointer("id = @request.auth.id")
		users.CreateRule, users.UpdateRule, users.DeleteRule = nil, nil, nil
		users.PasswordAuth.Enabled = false
		users.OTP.Enabled = false
		users.OAuth2.Enabled = true
		users.AuthAlert.Enabled = false
		users.Fields.Add(&core.SelectField{Name: "role", Values: []string{"investor", "admin"}, MaxSelect: 1, Required: true})
		if err := app.Save(users); err != nil {
			return err
		}

		save := func(c *core.Collection) error { return app.Save(c) }
		date := func(name string, required bool) *core.TextField {
			return &core.TextField{Name: name, Required: required, Pattern: `^\d{4}-\d{2}-\d{2}$`, Max: 10}
		}
		stamps := []core.Field{&core.AutodateField{Name: "created", OnCreate: true}, &core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true}}

		// ---- people who hold units ----
		investors := core.NewBaseCollection("investors")
		investors.Fields.Add(
			&core.EmailField{Name: "email", Required: true},
			&core.TextField{Name: "name", Required: true, Max: 120},
			&core.TextField{Name: "folio", Max: 30},
			&core.TextField{Name: "programme", Max: 40},
			&core.BoolField{Name: "inactive"},
			&core.TextField{Name: "note", Max: 500},
		)
		investors.Fields.Add(stamps...)
		investors.AddIndex("idx_investors_email", true, "email COLLATE NOCASE", "")
		if err := save(investors); err != nil {
			return err
		}

		txns := core.NewBaseCollection("investor_txns")
		txns.Fields.Add(
			&core.RelationField{Name: "investor", CollectionId: investors.Id, MaxSelect: 1, Required: true, CascadeDelete: true},
			date("date", true),
			&core.SelectField{Name: "kind", Values: []string{"subscription", "redemption", "transfer_in", "transfer_out"}, MaxSelect: 1, Required: true},
			// Rupees paid in (subscription) or out (redemption). Transfers move units without money.
			&core.NumberField{Name: "amount"},
			// Units allotted or given up. Leave 0 to price them at that day's NAV.
			&core.NumberField{Name: "units"},
			&core.TextField{Name: "note", Max: 300},
		)
		txns.Fields.Add(stamps...)
		txns.AddIndex("idx_txns_investor", false, "investor", "")
		if err := save(txns); err != nil {
			return err
		}

		// Money into / out of the fund as a whole. NAV = portfolio value / units from these rows.
		flows := core.NewBaseCollection("capital_flows")
		flows.Fields.Add(
			date("date", true),
			&core.NumberField{Name: "amount", Required: true},
			&core.NumberField{Name: "units"},
			&core.TextField{Name: "note", Max: 300},
		)
		flows.Fields.Add(stamps...)
		if err := save(flows); err != nil {
			return err
		}

		// ---- synced from the tracker (replaced on every sync) ----
		trades := core.NewBaseCollection("trades")
		trades.Fields.Add(
			&core.TextField{Name: "tradeId", Required: true, Max: 40},
			&core.NumberField{Name: "row", OnlyInt: true},
			date("date", true),
			&core.TextField{Name: "pod", Max: 40},
			&core.TextField{Name: "symbol", Required: true, Max: 40},
			&core.TextField{Name: "name", Max: 160},
			&core.NumberField{Name: "qty", Required: true},
			&core.NumberField{Name: "price", Required: true},
			&core.TextField{Name: "stoploss", Max: 80},
			&core.TextField{Name: "target", Max: 80},
			&core.TextField{Name: "sector", Max: 80},
			&core.TextField{Name: "horizon", Max: 80},
			&core.TextField{Name: "thesis", Max: 4000},
		)
		trades.AddIndex("idx_trades_date", false, "date", "")
		if err := save(trades); err != nil {
			return err
		}

		other := core.NewBaseCollection("other_pnl")
		other.Fields.Add(
			&core.TextField{Name: "book", Max: 80},
			date("date", true),
			&core.NumberField{Name: "gross"},
			&core.NumberField{Name: "charges"},
			&core.NumberField{Name: "net"},
			&core.NumberField{Name: "row", OnlyInt: true},
		)
		if err := save(other); err != nil {
			return err
		}

		instruments := core.NewBaseCollection("instruments")
		instruments.Fields.Add(
			&core.TextField{Name: "symbol", Required: true, Max: 40},
			&core.TextField{Name: "name", Max: 160},
			&core.TextField{Name: "exchange", Max: 20},
			&core.TextField{Name: "type", Max: 20},
			&core.TextField{Name: "industry", Max: 120},
			// Our own sector label, shown in allocation. Filled from the tracker's Industry column; edit to override.
			&core.TextField{Name: "sector", Max: 80},
			&core.BoolField{Name: "sectorLocked"},
			// Yahoo Finance symbol for price history, e.g. HDFCBANK.NS. Blank: <symbol>.NS.
			&core.TextField{Name: "yahoo", Max: 40},
			&core.NumberField{Name: "lastPrice"},
			&core.NumberField{Name: "prevClose"},
			&core.TextField{Name: "priceAt", Max: 40},
		)
		instruments.Fields.Add(stamps...)
		instruments.AddIndex("idx_instruments_symbol", true, "symbol", "")
		if err := save(instruments); err != nil {
			return err
		}

		prices := core.NewBaseCollection("prices")
		prices.Fields.Add(
			&core.TextField{Name: "symbol", Required: true, Max: 40},
			date("day", true),
			&core.NumberField{Name: "close", Required: true},
			// excel (tracker snapshot) < yahoo (history) < manual (typed in the dashboard, never overwritten)
			&core.SelectField{Name: "source", Values: []string{"excel", "yahoo", "manual"}, MaxSelect: 1},
		)
		prices.AddIndex("idx_prices_symbol_day", true, "symbol, day", "")
		if err := save(prices); err != nil {
			return err
		}

		// Computed after every sync (kept for export and the dashboard).
		nav := core.NewBaseCollection("nav")
		nav.Fields.Add(
			date("day", true),
			&core.NumberField{Name: "nav"},
			&core.NumberField{Name: "cash"},
			&core.NumberField{Name: "holdings"},
			&core.NumberField{Name: "value"},
			&core.NumberField{Name: "units"},
			&core.NumberField{Name: "nifty50"},
			&core.NumberField{Name: "nifty500"},
			&core.NumberField{Name: "positions", OnlyInt: true},
		)
		nav.AddIndex("idx_nav_day", true, "day", "")
		if err := save(nav); err != nil {
			return err
		}

		syncLog := core.NewBaseCollection("sync_log")
		syncLog.Fields.Add(
			&core.TextField{Name: "source", Max: 40}, // link | upload | cli | prices
			&core.BoolField{Name: "ok"},
			&core.TextField{Name: "message", Max: 2000},
			&core.NumberField{Name: "trades", OnlyInt: true},
			&core.JSONField{Name: "warnings", MaxSize: 200000},
			&core.TextField{Name: "by", Max: 200},
		)
		syncLog.Fields.Add(stamps...)
		syncLog.AddIndex("idx_sync_created", false, "created", "")
		if err := save(syncLog); err != nil {
			return err
		}

		// ---- editorial ----
		reports := core.NewBaseCollection("reports")
		reports.Fields.Add(
			&core.TextField{Name: "slug", Required: true, Pattern: `^[a-z0-9]+(?:-[a-z0-9]+)*$`, Max: 80},
			&core.TextField{Name: "title", Required: true, Max: 200},
			// Shown as the kind, e.g. "Quarterly Letter", "Monthly Factsheet", "Research", "Macro Note".
			&core.TextField{Name: "type", Required: true, Max: 60},
			&core.SelectField{Name: "category", Values: []string{"Letters", "Factsheets", "Research", "Macro"}, MaxSelect: 1, Required: true},
			date("date", true),
			&core.SelectField{Name: "access", Values: []string{"public", "investors"}, MaxSelect: 1, Required: true},
			&core.TextField{Name: "author", Max: 120},
			&core.TextField{Name: "dek", Max: 600},     // italic standfirst under the title
			&core.TextField{Name: "summary", Max: 600}, // card blurb on the publications page
			&core.TextField{Name: "kicker", Max: 40},   // big gold word on the banner, e.g. "Q1 FY27"
			&core.TextField{Name: "kickerSub", Max: 80},
			// Key figures beside the text: [{"k":"FUND · Q1 FY27","v":"+7.4%","up":true}]
			&core.JSONField{Name: "facts", MaxSize: 5000},
			// The article in Markdown. See README "Writing a report".
			&core.TextField{Name: "body", Max: 200000},
			&core.FileField{Name: "pdf", MaxSelect: 1, MaxSize: 50 << 20, MimeTypes: []string{"application/pdf"}, Protected: true},
			&core.TextField{Name: "pages", Max: 20},
			&core.BoolField{Name: "published"},
		)
		reports.Fields.Add(stamps...)
		reports.AddIndex("idx_reports_slug", true, "slug", "")
		if err := save(reports); err != nil {
			return err
		}

		team := core.NewBaseCollection("team")
		team.ListRule = types.Pointer("") // public, so photos can be served
		team.ViewRule = types.Pointer("")
		team.Fields.Add(
			&core.TextField{Name: "name", Required: true, Max: 120},
			&core.TextField{Name: "role", Required: true, Max: 120},
			&core.SelectField{Name: "group", Values: []string{"leadership", "pgp2", "pgp1"}, MaxSelect: 1, Required: true},
			&core.TextField{Name: "org", Max: 120},
			&core.TextField{Name: "batch", Max: 40},
			&core.NumberField{Name: "order", OnlyInt: true},
			&core.FileField{Name: "photo", MaxSelect: 1, MaxSize: 8 << 20, MimeTypes: []string{"image/jpeg", "image/png", "image/webp"}, Thumbs: []string{"600x750", "400x400"}},
			&core.URLField{Name: "linkedin"},
		)
		if err := save(team); err != nil {
			return err
		}

		// One row of fund-wide settings.
		settings := core.NewBaseCollection("fund_settings")
		settings.Fields.Add(
			&core.NumberField{Name: "startNav"},
			&core.NumberField{Name: "riskFree"}, // % a year, e.g. 6.5
			&core.TextField{Name: "managerNote", Max: 2000},
			&core.TextField{Name: "managerNoteBy", Max: 120},
			date("managerNoteDate", false),
			&core.TextField{Name: "motto", Max: 80},
		)
		if err := save(settings); err != nil {
			return err
		}
		row := core.NewRecord(settings)
		row.Set("startNav", 1000)
		row.Set("riskFree", 6.5)
		row.Set("motto", "Per ardua ad alta")
		return app.Save(row)
	}, nil)
}
