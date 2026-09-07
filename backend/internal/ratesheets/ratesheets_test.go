package ratesheets

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestParseTabularCSV(t *testing.T) {
	raw := []byte("courier,service,zone,weight,rate\nFedEx,Duty paid premium,7,0-5 kg,18800\nDHL,Express,6,5-10 kg,22000\n")

	rows, err := Parse(raw, "rates.csv", "", "")
	if err != nil {
		t.Fatalf("parse tabular csv: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Courier != "FedEx" || rows[0].Service != "Duty paid premium" || rows[0].Zone != "7" || rows[0].Price != 18800 {
		t.Fatalf("unexpected first row: %#v", rows[0])
	}
}

func TestParseZoneMatrixCSV(t *testing.T) {
	raw := []byte("weight,zone 1,zone 2,zone 7\n0-5 kg,1000,1200,18800\n5-10 kg,1500,1600,21000\n")

	rows, err := Parse(raw, "seven-zone.csv", "FedEx", "Duty paid premium")
	if err != nil {
		t.Fatalf("parse matrix csv: %v", err)
	}
	if len(rows) != 6 {
		t.Fatalf("expected 6 rows, got %d", len(rows))
	}
	if rows[2].Courier != "FedEx" || rows[2].Zone != "7" || rows[2].Weight != "0-5 kg" || rows[2].Price != 18800 {
		t.Fatalf("unexpected zone 7 row: %#v", rows[2])
	}
}

func TestParseXLSX(t *testing.T) {
	raw := minimalXLSX(t)
	rows, err := Parse(raw, "rates.xlsx", "UPS", "Export saver")
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[1].Courier != "UPS" || rows[1].Zone != "2" || rows[1].Price != 1200 {
		t.Fatalf("unexpected xlsx row: %#v", rows[1])
	}
}

func TestPostalPrefixUsesCountryFormat(t *testing.T) {
	tests := []struct {
		name    string
		postal  string
		country string
		want    string
	}{
		{name: "US ZIP", postal: "10001-1200", country: "US", want: "100"},
		{name: "Canadian postal", postal: "M5V 3A8", country: "CA", want: "M5V"},
		{name: "UK postcode", postal: "SW1A 1AA", country: "GB", want: "SW1"},
		{name: "too short", postal: "A1", country: "CA", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := postalPrefix(test.postal, test.country); got != test.want {
				t.Fatalf("postalPrefix(%q, %q) = %q, want %q", test.postal, test.country, got, test.want)
			}
		})
	}
}

func minimalXLSX(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	addZipFile(t, writer, "xl/sharedStrings.xml", `<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <si><t>weight</t></si><si><t>zone 1</t></si><si><t>zone 2</t></si><si><t>0-5 kg</t></si>
</sst>`)
	addZipFile(t, writer, "xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="C1" t="s"><v>2</v></c></row>
    <row r="2"><c r="A2" t="s"><v>3</v></c><c r="B2"><v>1000</v></c><c r="C2"><v>1200</v></c></row>
  </sheetData>
</worksheet>`)
	if err := writer.Close(); err != nil {
		t.Fatalf("close xlsx: %v", err)
	}
	return buffer.Bytes()
}

func addZipFile(t *testing.T, writer *zip.Writer, name string, content string) {
	t.Helper()
	file, err := writer.Create(name)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	if _, err := file.Write([]byte(content)); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
}
