package fund

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// Guest is someone who may sign in to read the portfolio without holding units.
type Guest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Note    string `json:"note"`
	Created string `json:"created"`
}

func (s *Service) Guests() ([]Guest, error) {
	rows, err := s.app.FindRecordsByFilter("guests", "", "name", 0, 0)
	if err != nil {
		return nil, err
	}
	out := make([]Guest, 0, len(rows))
	for _, r := range rows {
		out = append(out, Guest{r.Id, r.GetString("name"), r.GetString("email"), r.GetString("note"), r.GetDateTime("created").String()})
	}
	return out, nil
}

func (s *Service) guestFor(email string) *core.Record {
	r, _ := s.app.FindFirstRecordByFilter("guests", "email = {:e}", map[string]any{"e": strings.ToLower(strings.TrimSpace(email))})
	return r
}

// SaveGuest adds a guest, or renames one already on the list.
func (s *Service) SaveGuest(g Guest) (*core.Record, error) {
	g.Name, g.Note = strings.TrimSpace(g.Name), strings.TrimSpace(g.Note)
	g.Email = strings.ToLower(strings.TrimSpace(g.Email))
	if g.Name == "" {
		return nil, fmt.Errorf("enter the guest's name")
	}
	if a, err := mail.ParseAddress(g.Email); err != nil || a.Address != g.Email {
		return nil, fmt.Errorf("%q is not an email address", g.Email)
	}
	if inv := s.InvestorFor(g.Email); inv != nil {
		return nil, fmt.Errorf("%s is already an investor and can sign in", g.Email)
	}
	r := s.guestFor(g.Email)
	if r == nil {
		c, err := s.app.FindCollectionByNameOrId("guests")
		if err != nil {
			return nil, err
		}
		r = core.NewRecord(c)
	}
	r.Load(map[string]any{"name": g.Name, "email": g.Email, "note": g.Note})
	return r, s.app.Save(r)
}

// DeleteGuest removes a guest and signs them out by deleting their account.
func (s *Service) DeleteGuest(id string) error {
	r, err := s.app.FindRecordById("guests", id)
	if err != nil {
		return fmt.Errorf("that guest no longer exists")
	}
	return s.app.RunInTransaction(func(tx core.App) error {
		if u, err := tx.FindAuthRecordByEmail("users", r.GetString("email")); err == nil && u.GetString("role") == "guest" {
			if err := tx.Delete(u); err != nil {
				return err
			}
		}
		return tx.Delete(r)
	})
}
