package fund

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

// Commands adds the admin CLI:
//
//	auxesis-server import tracker.xlsx    import the trade log from a file
//	auxesis-server sync                   download the tracker from EXCEL_URL and import it
//	auxesis-server prices [--full]        download daily closes from Yahoo Finance
//	auxesis-server flow 2026-07-13 1238342.20 --units 1238.3422 --note "Initial corpus"
//	auxesis-server seed [--demo-prices]   sample reports and investors for local development
func Commands(app *pocketbase.PocketBase, s *Service) []*cobra.Command {
	boot := func() error {
		if err := app.Bootstrap(); err != nil {
			return err
		}
		return app.RunAllMigrations()
	}

	imp := &cobra.Command{
		Use:   "import <tracker.xlsx>",
		Short: "Import the trade log from a tracker workbook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := boot(); err != nil {
				return err
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			sum, err := s.Import(data, "cli", "")
			if err != nil {
				return err
			}
			printSummary(sum)
			return nil
		},
	}

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Download the tracker from EXCEL_URL and import it",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := boot(); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			sum, err := s.SyncFromLink(ctx, "cli")
			if err != nil {
				return err
			}
			printSummary(sum)
			return nil
		},
	}

	var full bool
	prices := &cobra.Command{
		Use:   "prices",
		Short: "Download daily closing prices (Yahoo Finance) for every instrument and both benchmarks",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := boot(); err != nil {
				return err
			}
			n, errs := s.BackfillPrices(context.Background(), full)
			fmt.Printf("Stored %d daily closes.\n", n)
			for _, e := range errs {
				fmt.Println("  !", e)
			}
			return s.Recompute()
		},
	}
	prices.Flags().BoolVar(&full, "full", false, "re-download everything since inception")

	var units float64
	var note string
	flow := &cobra.Command{
		Use:   "flow <date YYYY-MM-DD> <amount>",
		Short: "Record money into (+) or out of (−) the fund",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := boot(); err != nil {
				return err
			}
			if _, err := time.Parse("2006-01-02", args[0]); err != nil {
				return fmt.Errorf("date must look like 2026-07-13")
			}
			amt, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return fmt.Errorf("amount must be a number")
			}
			c, err := app.FindCollectionByNameOrId("capital_flows")
			if err != nil {
				return err
			}
			r := core.NewRecord(c)
			r.Load(map[string]any{"date": args[0], "amount": amt, "units": units, "note": note})
			if err := app.Save(r); err != nil {
				return err
			}
			fmt.Println("Saved. NAV will be recomputed on the next request.")
			return nil
		},
	}
	flow.Flags().Float64Var(&units, "units", 0, "units issued (0: priced at the previous day's NAV)")
	flow.Flags().StringVar(&note, "note", "", "a note")

	var demoPrices bool
	seed := &cobra.Command{
		Use:   "seed",
		Short: "Add sample reports and investors for local development (never on the live server)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := boot(); err != nil {
				return err
			}
			if err := Seed(app, demoPrices); err != nil {
				return err
			}
			fmt.Println("Seeded sample data.")
			return s.Recompute()
		},
	}
	seed.Flags().BoolVar(&demoPrices, "demo-prices", false, "fill missing daily prices with a synthetic random walk (for offline development)")

	return []*cobra.Command{imp, syncCmd, prices, flow, seed}
}

func printSummary(sum *SyncSummary) {
	fmt.Println(sum.Message)
	for _, w := range sum.Warnings {
		fmt.Println("  !", w)
	}
}

// Seed adds sample content for local development. It skips any collection that already has rows.
func Seed(app core.App, demoPrices bool) error {
	count := func(c string) int64 { n, _ := app.CountRecords(c); return n }
	add := func(c string, data map[string]any) (*core.Record, error) {
		col, err := app.FindCollectionByNameOrId(c)
		if err != nil {
			return nil, err
		}
		r := core.NewRecord(col)
		r.Load(data)
		return r, app.Save(r)
	}

	if count("capital_flows") == 0 {
		if _, err := add("capital_flows", map[string]any{"date": "2026-07-13", "amount": 1238342.2, "units": 1238.3422, "note": "Sample corpus"}); err != nil {
			return err
		}
	}
	if count("investors") == 0 {
		for i, inv := range []struct {
			email, name, prog string
			amt               float64
		}{
			{"investor@gmail.com", "Sample Investor", "PGP1", 50000}, {"second.investor@gmail.com", "Second Investor", "PGP2", 20000},
		} {
			r, err := add("investors", map[string]any{"email": inv.email, "name": inv.name, "folio": fmt.Sprintf("AUX-%04d", 142+i), "programme": inv.prog})
			if err != nil {
				return err
			}
			if _, err := add("investor_txns", map[string]any{"investor": r.Id, "date": "2026-07-13", "kind": "subscription", "amount": inv.amt, "nav": 1000, "units": inv.amt / 1000}); err != nil {
				return err
			}
		}
	}
	if count("reports") == 0 {
		for _, r := range sampleReports {
			if _, err := add("reports", r); err != nil {
				return err
			}
		}
	}
	if rows, _ := app.FindRecordsByFilter("fund_settings", "", "", 1, 0); len(rows) > 0 && rows[0].GetString("managerNote") == "" {
		rows[0].Load(map[string]any{"managerNote": "We added to industrials on the September correction and trimmed consumer names where valuations ran ahead of earnings. Cash stays near three per cent.", "managerNoteBy": "Fund Manager", "managerNoteDate": "2026-09-22"})
		if err := app.Save(rows[0]); err != nil {
			return err
		}
	}
	if demoPrices {
		return seedDemoPrices(app)
	}
	return nil
}

// seedDemoPrices fills every weekday since inception with a synthetic close for each
// instrument, walking from its first trade price to its latest known price. Real prices
// (Yahoo) replace these on the next download.
func seedDemoPrices(app core.App) error {
	var s Service
	s.app = app
	trades, _, _, prices, _, err := s.loadInputs()
	if err != nil {
		return err
	}
	if len(trades) == 0 {
		return fmt.Errorf("import a tracker first")
	}
	start := dateDay(trades[0].Date)
	for _, t := range trades {
		if d := dateDay(t.Date); d < start {
			start = d
		}
	}
	days := weekdays(start, today())
	anchors := map[string]map[Day]float64{}
	for _, t := range trades {
		if anchors[t.Symbol] == nil {
			anchors[t.Symbol] = map[Day]float64{}
		}
		anchors[t.Symbol][dateDay(t.Date)] = t.Price
	}
	for sym, m := range prices {
		for d, v := range m {
			if anchors[sym] == nil {
				anchors[sym] = map[Day]float64{}
			}
			anchors[sym][d] = v
		}
	}
	if len(anchors[Nifty50]) == 0 {
		anchors[Nifty50] = map[Day]float64{start: 25150, days[len(days)-1]: 24350}
	}
	if len(anchors[Nifty500]) == 0 {
		anchors[Nifty500] = map[Day]float64{start: 23300, days[len(days)-1]: 22550}
	}
	rng := rand.New(rand.NewSource(7))
	return app.RunInTransaction(func(tx core.App) error {
		for sym, a := range anchors {
			var ks []Day
			for d := range a {
				ks = append(ks, d)
			}
			sort.Strings(ks)
			walk := 0.0
			for _, d := range days {
				if _, ok := prices[sym][d]; ok {
					continue
				}
				i := sort.SearchStrings(ks, d)
				var v float64
				switch {
				case i < len(ks) && ks[i] == d:
					v = a[d]
				case i == 0:
					v = a[ks[0]]
				case i == len(ks):
					v = a[ks[len(ks)-1]]
				default:
					d0, d1 := ks[i-1], ks[i]
					t0, _ := time.Parse("2006-01-02", d0)
					t1, _ := time.Parse("2006-01-02", d1)
					td, _ := time.Parse("2006-01-02", d)
					f := td.Sub(t0).Hours() / t1.Sub(t0).Hours()
					v = a[d0] * math.Pow(a[d1]/a[d0], f)
				}
				walk = walk*0.8 + rng.NormFloat64()*0.008
				v *= 1 + walk
				if err := insertPriceIfMissing(tx, sym, d, math.Round(v*100)/100); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
