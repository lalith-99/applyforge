package jobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// UnmarshalJSON keeps the employers-list importer tolerant of the upstream
// compact JSON representation. The upstream fiscal-year field is enrichment
// metadata and has appeared as a JSON string in addition to a JSON number.
func (row *employerListRow) UnmarshalJSON(data []byte) error {
	type rowAlias employerListRow
	var payload struct {
		rowAlias
		FiscalYear json.RawMessage `json:"fy"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	*row = employerListRow(payload.rowAlias)
	fiscalYear, err := decodeEmployerListFiscalYear(payload.FiscalYear)
	if err != nil {
		return err
	}
	row.FiscalYear = fiscalYear
	return nil
}

func decodeEmployerListFiscalYear(raw json.RawMessage) (*int, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}

	var number int
	if err := json.Unmarshal(raw, &number); err == nil {
		return &number, nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return nil, fmt.Errorf("decode employers-list fiscal year: expected number or string: %w", err)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}

	// Accept the common compact forms "2023", "FY2023", and "FY 2023" while
	// still rejecting unrelated text so a real upstream schema change is visible.
	normalized := strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(text), "FY"))
	number, err := strconv.Atoi(normalized)
	if err != nil {
		return nil, fmt.Errorf("decode employers-list fiscal year %q: %w", text, err)
	}
	return &number, nil
}
