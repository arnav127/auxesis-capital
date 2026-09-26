package fund

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math"
	"strings"
)

// ImportRow is an investor read from a workbook, for the admin to check before saving.
type ImportRow struct {
	InvestorInput
	Sheet  string `json:"sheet"`
	Row    int    `json:"row"`
	Issue  string `json:"issue,omitempty"` // why this row needs a look before importing
	Action string `json:"action,omitempty"`
}

// InvestorImport is the preview of a workbook's investor list.
type InvestorImport struct {
	Sheet    string      `json:"sheet"`
	Rows     []ImportRow `json:"rows"`
	Units    float64     `json:"units"`
	Amount   float64     `json:"amount"`
	Warnings []string    `json:"warnings"`
}

var investorAliases = map[string][]string{
	"email":     {"email", "emailaddress", "emailid", "gmail", "mail"},
	"name":      {"name", "investorname", "fullname"},
	"student":   {"studentname"},
	"programme": {"programme", "program", "cohort", "batch"},
	"amount":    {"invested", "amount", "investment", "investmentamount", "amountinvested", "loanamount", "contribution"},
	"nav":       {"nav", "allotmentnav", "navatallotment"},
	"units":     {"newupdatedunits", "newunits", "currentunits", "units", "unitsallotted", "unitscredited", "oldunits"},
	"note":      {"note", "notes", "remarks"},
	"date":      {"date", "allotmentdate", "dateofallotment"},
	"folio":     {"folio", "foliono", "folionumber"},
	"action":    {"action", "status"},
	"toName":    {"transfereename"},
	"toEmail":   {"transfereeemail", "transfereeemailid"},
	"toBatch":   {"transfereebatch"},
}

// ParseInvestors reads an investor list from a CSV file or from the first workbook sheet that has
// an email column plus a name and units or amount column (in the fund's tracker:
// "Investors-Carry Forwarded Data").
func ParseInvestors(data []byte) (*InvestorImport, error) {
	if !bytes.HasPrefix(data, []byte("PK")) {
		sh, err := csvSheet(data)
		if err != nil {
			return nil, err
		}
		for _, r := range sh.rowNums() {
			if r > 30 {
				break
			}
			if cols := matchColumns(sh.rows[r]); isInvestorHeader(cols) {
				return readInvestors(nil, "CSV", sh, r, cols), nil
			}
		}
		return nil, fmt.Errorf("this file has no investor list (a header row with Email, Name and Units or Amount)")
	}
	wb, err := openWorkbook(data)
	if err != nil {
		return nil, err
	}
	for _, name := range wb.sheetNames {
		sh, err := wb.sheet(name)
		if err != nil {
			continue
		}
		for _, r := range sh.rowNums() {
			if r > 30 {
				break
			}
			if cols := matchColumns(sh.rows[r]); isInvestorHeader(cols) {
				return readInvestors(wb, name, sh, r, cols), nil
			}
		}
	}
	return nil, fmt.Errorf("no sheet in this file has an investor list (a header row with Email, Name and Units or Amount)")
}

func isInvestorHeader(cols map[string]int) bool {
	_, hasEmail := cols["email"]
	_, hasName := cols["name"]
	_, hasUnits := cols["units"]
	_, hasAmount := cols["amount"]
	return hasEmail && hasName && (hasUnits || hasAmount)
}

// csvSheet loads a CSV file (e.g. a Google Sheets download) as a sheet of text cells.
func csvSheet(data []byte) (*sheet, error) {
	rd := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	rd.FieldsPerRecord = -1
	rd.LazyQuotes = true
	recs, err := rd.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("this is not a CSV or .xlsx file (%v)", err)
	}
	sh := &sheet{rows: map[int]map[int]*cell{}}
	for i, rec := range recs {
		row := map[int]*cell{}
		for j, v := range rec {
			if v = strings.TrimSpace(v); v != "" {
				row[j] = &cell{typ: "str", val: v}
			}
		}
		sh.rows[i+1] = row
	}
	return sh, nil
}

// matchColumns maps our keys to column indexes. For units, the first matching alias in
// priority order wins (so "New Updated Units" beats "Old Units").
func matchColumns(row map[int]*cell) map[string]int {
	cols := map[string]int{}
	for key, aliases := range investorAliases {
		best := len(aliases)
		for c, cl := range row {
			h := normHeader(cl.text())
			for i, a := range aliases {
				if h == a && i < best {
					best, cols[key] = i, c
				}
			}
		}
	}
	return cols
}

func readInvestors(_ *workbook, name string, sh *sheet, hr int, cols map[string]int) *InvestorImport {
	out := &InvestorImport{Sheet: name, Rows: []ImportRow{}}
	get := func(r int, key string) *cell {
		c, ok := cols[key]
		if !ok {
			return nil
		}
		return sh.rows[r][c]
	}
	text := func(r int, key string) string {
		c := get(r, key)
		if c == nil || c.isError() {
			return ""
		}
		return strings.TrimSpace(c.text())
	}
	num := func(r int, key string) float64 {
		v, _ := get(r, key).number()
		return v
	}
	for _, r := range sh.rowNums() {
		if r <= hr {
			continue
		}
		email := strings.ToLower(text(r, "email"))
		nm := text(r, "name")
		if nm == "" && (email == "" || !strings.Contains(email, "@")) {
			continue // blank lines, totals and notes below the list
		}
		row := ImportRow{Sheet: name, Row: r, InvestorInput: InvestorInput{Name: nm, Email: email, Programme: text(r, "programme"), Folio: text(r, "folio")}}
		if row.Name == "" {
			row.Name = text(r, "student")
		}
		row.NAV = num(r, "nav")
		if row.NAV <= 0 {
			row.NAV = 1000
		}
		units, amount := num(r, "units"), num(r, "amount")
		// Units are what the investor holds now; the rupee value put in the fund is units × NAV.
		// (For carried-forward money that differs from the original amount, which goes in the note.)
		if units > 0 {
			row.Units = math.Round(units*1e6) / 1e6
			row.Amount = math.Round(units*row.NAV*100) / 100
			if amount > 0 && math.Abs(amount-row.Amount) > 0.5 {
				row.Note = fmt.Sprintf("Carried forward: originally ₹%s", inrPlain(amount))
			}
		} else {
			row.Amount = amount
		}
		if d, ok := get(r, "date").date(); ok {
			row.Date = d.Format("2006-01-02")
		}
		if n := text(r, "note"); n != "" {
			row.Note = n
		}
		row.Action = text(r, "action")
		if strings.EqualFold(row.Action, "transfer") {
			from := row.Name
			row.Name = text(r, "toName")
			row.Email = strings.ToLower(text(r, "toEmail"))
			if b := text(r, "toBatch"); b != "" {
				row.Programme = b
			}
			note := "Transferred from " + from
			if row.Note != "" {
				note = row.Note + " · " + note
			}
			row.Note = note
			if row.Email == "" {
				row.Issue = fmt.Sprintf("Units moved from %s to %s: add %s's Google email.", from, row.Name, firstWord(row.Name))
			}
		}
		if row.Email == "" && row.Issue == "" {
			row.Issue = "Add " + firstWord(row.Name) + "'s Google email."
		}
		if row.Amount <= 0 && row.Issue == "" {
			row.Issue = "No units or amount on this row."
		}
		out.Units += row.Units
		out.Amount += row.Amount
		out.Rows = append(out.Rows, row)
	}
	out.Units = math.Round(out.Units*1e6) / 1e6
	return out
}

func firstWord(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return s
}

// inrPlain formats 1234567 as 12,34,567.
func inrPlain(x float64) string {
	s := fmt.Sprintf("%.0f", math.Abs(x))
	if len(s) > 3 {
		head, tail := s[:len(s)-3], s[len(s)-3:]
		var parts []string
		for len(head) > 2 {
			parts = append([]string{head[len(head)-2:]}, parts...)
			head = head[:len(head)-2]
		}
		if head != "" {
			parts = append([]string{head}, parts...)
		}
		s = strings.Join(parts, ",") + "," + tail
	}
	if x < 0 {
		return "-" + s
	}
	return s
}
