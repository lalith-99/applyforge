package jobs

import "testing"

func TestDecodeEmployerListDatasetAcceptsStringFiscalYear(t *testing.T) {
	dataset, err := decodeEmployerListDataset([]byte(`{
		"updated":"2026-09-15",
		"rows":[{
			"n":"Example Corp",
			"a":42,
			"fy":"2023"
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(dataset.Rows) != 1 || dataset.Rows[0].FiscalYear == nil || *dataset.Rows[0].FiscalYear != 2023 {
		t.Fatalf("unexpected decoded fiscal year: %+v", dataset.Rows)
	}
}

func TestDecodeEmployerListFiscalYearFormats(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want *int
	}{
		{name: "number", raw: `2023`, want: intPtr(2023)},
		{name: "numeric string", raw: `"2023"`, want: intPtr(2023)},
		{name: "FY prefix", raw: `"FY2023"`, want: intPtr(2023)},
		{name: "FY prefix with space", raw: `"FY 2023"`, want: intPtr(2023)},
		{name: "blank", raw: `""`, want: nil},
		{name: "null", raw: `null`, want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := decodeEmployerListFiscalYear([]byte(test.raw))
			if err != nil {
				t.Fatal(err)
			}
			if test.want == nil {
				if got != nil {
					t.Fatalf("got %v, want nil", *got)
				}
				return
			}
			if got == nil || *got != *test.want {
				t.Fatalf("got %v, want %d", got, *test.want)
			}
		})
	}
}

func TestDecodeEmployerListFiscalYearRejectsUnexpectedText(t *testing.T) {
	if _, err := decodeEmployerListFiscalYear([]byte(`"unknown"`)); err == nil {
		t.Fatal("expected invalid fiscal-year text to fail decoding")
	}
}

func intPtr(value int) *int {
	return &value
}
