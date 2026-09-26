package fund

import (
	"fmt"
	"math"
	"net/mail"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// InvestorInput is one investor as the admin page enters them: who they are and their allotment.
type InvestorInput struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	Folio     string  `json:"folio"`
	Programme string  `json:"programme"`
	Amount    float64 `json:"amount"` // rupees invested
	NAV       float64 `json:"nav"`    // NAV per unit at allotment (1000 at launch)
	Units     float64 `json:"units"`  // 0: amount / nav
	Date      string  `json:"date"`   // allotment date, YYYY-MM-DD
	Note      string  `json:"note"`
}

func (in *InvestorInput) clean(defaultDate string) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Folio, in.Programme, in.Date, in.Note = strings.TrimSpace(in.Folio), strings.TrimSpace(in.Programme), strings.TrimSpace(in.Date), strings.TrimSpace(in.Note)
	if in.Name == "" {
		return fmt.Errorf("a name is needed")
	}
	if a, err := mail.ParseAddress(in.Email); err != nil || a.Address != in.Email {
		return fmt.Errorf("%q is not an email address", in.Email)
	}
	if in.NAV == 0 {
		in.NAV = 1000
	}
	if in.Amount <= 0 || in.NAV <= 0 || in.Units < 0 {
		return fmt.Errorf("the amount and NAV must be positive")
	}
	if in.Units == 0 {
		in.Units = math.Round(in.Amount/in.NAV*1e4) / 1e4
	}
	if in.Date == "" {
		in.Date = defaultDate
	}
	if _, err := time.Parse("2006-01-02", in.Date); err != nil {
		return fmt.Errorf("the date %q should look like 2026-07-13", in.Date)
	}
	return nil
}

func (s *Service) defaultAllotDate() string {
	if d := s.inceptionDay(); d != "" {
		return d
	}
	return today()
}

// SaveInvestor creates or updates an investor and their allotment (their first subscription).
func (s *Service) SaveInvestor(in InvestorInput) (*core.Record, error) {
	if err := in.clean(s.defaultAllotDate()); err != nil {
		return nil, err
	}
	var saved *core.Record
	err := s.app.RunInTransaction(func(tx core.App) error {
		r, err := saveInvestor(tx, in)
		saved = r
		return err
	})
	if err == nil {
		s.Invalidate()
	}
	return saved, err
}

func saveInvestor(tx core.App, in InvestorInput) (*core.Record, error) {
	var inv *core.Record
	if in.ID != "" {
		r, err := tx.FindRecordById("investors", in.ID)
		if err != nil {
			return nil, fmt.Errorf("that investor no longer exists")
		}
		inv = r
	} else if r, _ := tx.FindFirstRecordByFilter("investors", "email = {:e}", map[string]any{"e": in.Email}); r != nil {
		inv = r
	}
	if other, _ := tx.FindFirstRecordByFilter("investors", "email = {:e}", map[string]any{"e": in.Email}); other != nil && inv != nil && other.Id != inv.Id {
		return nil, fmt.Errorf("%s is already registered to %s", in.Email, other.GetString("name"))
	}
	if inv == nil {
		c, err := tx.FindCollectionByNameOrId("investors")
		if err != nil {
			return nil, err
		}
		inv = core.NewRecord(c)
	}
	inv.Load(map[string]any{"name": in.Name, "email": in.Email, "folio": in.Folio, "programme": in.Programme})
	if err := tx.Save(inv); err != nil {
		return nil, err
	}

	rows, err := tx.FindRecordsByFilter("investor_txns", "investor = {:id} && kind = 'subscription'", "date,created", 1, 0, map[string]any{"id": inv.Id})
	if err != nil {
		return nil, err
	}
	var t *core.Record
	if len(rows) > 0 {
		t = rows[0]
	} else {
		c, err := tx.FindCollectionByNameOrId("investor_txns")
		if err != nil {
			return nil, err
		}
		t = core.NewRecord(c)
		t.Set("investor", inv.Id)
		t.Set("kind", "subscription")
		t.Set("note", "Allotment")
	}
	if in.Note != "" {
		t.Set("note", in.Note)
	}
	t.Load(map[string]any{"date": in.Date, "amount": in.Amount, "nav": in.NAV, "units": in.Units})
	if err := tx.Save(t); err != nil {
		return nil, err
	}
	return inv, nil
}

// SaveInvestors upserts many investors (matched by email) in one go; nothing is saved if any row is invalid.
func (s *Service) SaveInvestors(rows []InvestorInput) (int, error) {
	if len(rows) == 0 {
		return 0, fmt.Errorf("there are no rows to add")
	}
	d := s.defaultAllotDate()
	seen := map[string]int{}
	for i := range rows {
		rows[i].ID = ""
		if err := rows[i].clean(d); err != nil {
			return 0, fmt.Errorf("row %d: %v", i+1, err)
		}
		if j, dup := seen[rows[i].Email]; dup {
			return 0, fmt.Errorf("row %d: %s is also on row %d", i+1, rows[i].Email, j)
		}
		seen[rows[i].Email] = i + 1
	}
	err := s.app.RunInTransaction(func(tx core.App) error {
		for i, in := range rows {
			if _, err := saveInvestor(tx, in); err != nil {
				return fmt.Errorf("row %d: %v", i+1, err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	s.Invalidate()
	return len(rows), nil
}

// DeleteInvestor removes an investor and their transactions.
func (s *Service) DeleteInvestor(id string) error {
	r, err := s.app.FindRecordById("investors", id)
	if err != nil {
		return fmt.Errorf("that investor no longer exists")
	}
	if err := s.app.Delete(r); err != nil {
		return err
	}
	s.Invalidate()
	return nil
}

// ---------- fund capital ----------

type FlowRow struct {
	ID     string  `json:"id"`
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
	Units  float64 `json:"units"`
	Note   string  `json:"note"`
}

func (s *Service) Flows() ([]FlowRow, error) {
	rows, err := s.app.FindRecordsByFilter("capital_flows", "", "date,created", 0, 0)
	if err != nil {
		return nil, err
	}
	out := []FlowRow{}
	for _, r := range rows {
		out = append(out, FlowRow{ID: r.Id, Date: r.GetString("date"), Amount: r.GetFloat("amount"), Units: r.GetFloat("units"), Note: r.GetString("note")})
	}
	return out, nil
}

// AddFlow records money into (+) or out of (−) the fund as a whole.
func (s *Service) AddFlow(f FlowRow) error {
	if _, err := time.Parse("2006-01-02", f.Date); err != nil {
		return fmt.Errorf("the date should look like 2026-07-13")
	}
	if f.Amount == 0 {
		return fmt.Errorf("enter the amount (negative for money paid out)")
	}
	c, err := s.app.FindCollectionByNameOrId("capital_flows")
	if err != nil {
		return err
	}
	r := core.NewRecord(c)
	r.Load(map[string]any{"date": f.Date, "amount": f.Amount, "units": f.Units, "note": strings.TrimSpace(f.Note)})
	if err := s.app.Save(r); err != nil {
		return err
	}
	s.Invalidate()
	return nil
}

func (s *Service) DeleteFlow(id string) error {
	r, err := s.app.FindRecordById("capital_flows", id)
	if err != nil {
		return fmt.Errorf("that entry no longer exists")
	}
	if err := s.app.Delete(r); err != nil {
		return err
	}
	s.Invalidate()
	return nil
}

// ImportSummary reports what an import added.
type ImportSummary struct {
	Message    string `json:"message"`
	Investors  int    `json:"investors"` // new people
	Allotments int    `json:"allotments"`
	Skipped    int    `json:"skipped"` // already imported
}

// ImportInvestors adds each row as an allotment (a subscription) for the investor with that email,
// creating investors as needed. A person can have several rows (e.g. new money plus a carried-forward
// holding). Rows already in the database (same person, date, amount and units) are skipped, so the
// same file can be imported twice safely. Nothing is saved if any row is invalid.
func (s *Service) ImportInvestors(rows []InvestorInput) (*ImportSummary, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("there are no rows to import")
	}
	d := s.defaultAllotDate()
	for i := range rows {
		if err := rows[i].clean(d); err != nil {
			return nil, fmt.Errorf("row %d (%s): %v", i+1, rows[i].Name, err)
		}
	}
	sum := &ImportSummary{}
	err := s.app.RunInTransaction(func(tx core.App) error {
		ic, err := tx.FindCollectionByNameOrId("investors")
		if err != nil {
			return err
		}
		tc, err := tx.FindCollectionByNameOrId("investor_txns")
		if err != nil {
			return err
		}
		for i, in := range rows {
			inv, _ := tx.FindFirstRecordByFilter("investors", "email = {:e}", map[string]any{"e": in.Email})
			if inv == nil {
				inv = core.NewRecord(ic)
				inv.Load(map[string]any{"name": in.Name, "email": in.Email, "folio": in.Folio, "programme": in.Programme})
				if err := tx.Save(inv); err != nil {
					return fmt.Errorf("row %d (%s): %v", i+1, in.Name, err)
				}
				sum.Investors++
			}
			existing, err := tx.FindRecordsByFilter("investor_txns", "investor = {:id} && kind = 'subscription' && date = {:d}", "", 0, 0, map[string]any{"id": inv.Id, "d": in.Date})
			if err != nil {
				return err
			}
			dup := false
			for _, t := range existing {
				if math.Abs(t.GetFloat("amount")-in.Amount) < 0.005 && math.Abs(t.GetFloat("units")-in.Units) < 1e-6 {
					dup = true
				}
			}
			if dup {
				sum.Skipped++
				continue
			}
			note := in.Note
			if note == "" {
				note = "Allotment"
			}
			t := core.NewRecord(tc)
			t.Load(map[string]any{"investor": inv.Id, "date": in.Date, "kind": "subscription", "amount": in.Amount, "nav": in.NAV, "units": in.Units, "note": note})
			if err := tx.Save(t); err != nil {
				return fmt.Errorf("row %d (%s): %v", i+1, in.Name, err)
			}
			sum.Allotments++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.Invalidate()
	sum.Message = fmt.Sprintf("Imported %d allotments for %d new investors.", sum.Allotments, sum.Investors)
	if sum.Skipped > 0 {
		sum.Message += fmt.Sprintf(" %d rows were already in and were skipped.", sum.Skipped)
	}
	return sum, nil
}
