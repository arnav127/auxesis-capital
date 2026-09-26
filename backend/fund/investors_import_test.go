package fund

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// simpleWorkbook writes one sheet of plain values (strings and numbers).
func simpleWorkbook(t *testing.T, sheet string, rows [][]any) []byte {
	t.Helper()
	var sb strings.Builder
	for r, row := range rows {
		fmt.Fprintf(&sb, `<row r="%d">`, r+1)
		for c, v := range row {
			ref := fmt.Sprintf("%c%d", 'A'+c, r+1)
			switch x := v.(type) {
			case nil:
			case string:
				fmt.Fprintf(&sb, `<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, x)
			default:
				fmt.Fprintf(&sb, `<c r="%s"><v>%v</v></c>`, ref, x)
			}
		}
		sb.WriteString("</row>")
	}
	files := map[string]string{
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="` + sheet + `" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` + sb.String() + `</sheetData></worksheet>`,
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for n, b := range files {
		w, _ := zw.Create(n)
		w.Write([]byte(b))
	}
	zw.Close()
	return buf.Bytes()
}

func TestParseInvestorsWorkbook(t *testing.T) {
	data := simpleWorkbook(t, "Investors", [][]any{
		{"E-Mail", "Name", "Programme", "Loan Amount (₹)", "Old Units", "New Updated Units", "Action", "Transferee Name", "Transferee Batch"},
		{"A@IIMA.ac.in", "Asha Rao", "PGP1", 20000, 20, 18.57483, "CarryForward"},
		{"b@gmail.com", "Bo Li", "PGP2", 10000, 10, 9.2874, "Transfer", "Cy Das", "PGP1"},
		{"c@gmail.com", "Cara", "PGP1", 5000},
		{nil, nil, nil, nil, "New Units", 33},
	})
	got, err := ParseInvestors(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 3 {
		t.Fatalf("rows %+v", got.Rows)
	}
	a := got.Rows[0]
	if a.Email != "a@iima.ac.in" || a.Units != 18.57483 || a.Amount != 18574.83 || a.NAV != 1000 || !strings.Contains(a.Note, "20,000") {
		t.Errorf("carry forward %+v", a)
	}
	b := got.Rows[1]
	if b.Name != "Cy Das" || b.Email != "" || b.Programme != "PGP1" || b.Issue == "" || !strings.Contains(b.Note, "Bo Li") {
		t.Errorf("transfer %+v", b)
	}
	if c := got.Rows[2]; c.Units != 0 || c.Amount != 5000 {
		t.Errorf("amount only %+v", c)
	}
	if _, err := ParseInvestors(simpleWorkbook(t, "X", [][]any{{"hello"}})); err == nil {
		t.Error("no investor sheet accepted")
	}
	if inrPlain(1238342.2) != "12,38,342" || inrPlain(500) != "500" {
		t.Error(inrPlain(1238342.2))
	}
}

func TestParseInvestorsCSV(t *testing.T) {
	csv := "\xef\xbb\xbfTimestamp,Email address,Full Name,Roll Number,Cohort,Investment Amount,Units Credited,Transaction ID,,\n" +
		"7/3/26 18:13,p25x@iima.ac.in,X Y,PGP1,PGP 2,10000,10,370034,,\n" +
		",,,,,,,,,\n" +
		",p25z@iima.ac.in,Zed,R1,PGP1,20000,18.57,18575,,98765\n"
	got, err := ParseInvestors([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 2 || got.Rows[0].Programme != "PGP 2" || got.Rows[0].Units != 10 || got.Rows[0].Amount != 10000 {
		t.Fatalf("%+v", got.Rows)
	}
	if z := got.Rows[1]; z.Units != 18.57 || z.Amount != 18570 || !strings.Contains(z.Note, "20,000") {
		t.Errorf("carry %+v", z)
	}
	// A clean list with its own Invested, Units, Date and Note columns.
	clean := "Name,Email,Programme,Invested,NAV,Units,Date,Note\nA,a@x.in,PGP1,18574.83,1000,18.574831,2026-07-13,Carried forward\n"
	got, err = ParseInvestors([]byte(clean))
	if err != nil || len(got.Rows) != 1 || got.Rows[0].Units != 18.574831 || got.Rows[0].Date != "2026-07-13" || got.Rows[0].Note != "Carried forward" {
		t.Fatalf("%v %+v", err, got)
	}
}
