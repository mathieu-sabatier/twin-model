package dsl

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"io"
	"strconv"
	"sync"
)

// Unit is one row of the symbol → UNECE (UN/CEFACT Rec 20) engineering-unit
// table. UnitID is the UNECE common code packed big-endian as the ASCII bytes of
// the code into an int32 (e.g. "CEL" = 0x43454C = 4408652); it is the value the
// OPC UA EUInformation.UnitId carries.
type Unit struct {
	Symbol      string // DSL symbol as written, e.g. "°C"
	Code        string // UNECE common code, e.g. "CEL" (informational)
	UnitID      int32  // EUInformation.UnitId
	DisplayName string // EUInformation.DisplayName text, e.g. "°C"
	Description string // EUInformation.Description text, e.g. "degree Celsius"
}

// uneceCSV is the official UN/CEFACT Rec 20 unit list, vendored verbatim from
// UA-ModelCompiler (Opc.Ua.ModelCompiler/CSVs/UNECE_to_OPCUA.csv) — the same
// source the ModelCompiler uses to generate EngineeringUnits. Columns:
// UNECECode,UnitId,DisplayName,Description. Relying on this file (rather than a
// hand-curated table) makes every standard unit symbol resolvable and keeps our
// UnitId values byte-aligned with the compiler's own output.
//
//go:embed UNECE_to_OPCUA.csv
var uneceCSV []byte

var (
	unitsOnce sync.Once
	units     map[string]Unit
)

// supplements cover symbols the vendored CSV omits, plus ergonomic ASCII aliases
// for units whose official DisplayName is awkward to type. "%" (Rec 20 code P1)
// is absent from UNECE_to_OPCUA.csv, so it is derived here (0x5031 = 20529).
// "kWh" aliases the CSV's "kW·h" (KWH) so authors need not type the middle dot.
var supplements = []Unit{
	{"%", "P1", 20529, "%", "percent"},
	{"kWh", "KWH", 4937544, "kW·h", "kilowatt hour"},
}

// loadUnits parses the embedded UNECE CSV into a DisplayName → Unit map. On a
// duplicate symbol the first row wins (the canonical/base unit is listed first).
// Supplements fill in symbols the CSV omits without overriding it.
func loadUnits() {
	units = make(map[string]Unit, 1400)
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(uneceCSV, []byte("\ufeff"))))
	r.FieldsPerRecord = -1 // tolerate the odd short/long row rather than aborting
	first := true
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(rec) < 4 {
			continue
		}
		if first { // header: UNECECode,UnitId,DisplayName,Description
			first = false
			continue
		}
		code, idStr, disp, desc := rec[0], rec[1], rec[2], rec[3]
		id, err := strconv.Atoi(idStr)
		if err != nil || disp == "" {
			continue
		}
		if _, seen := units[disp]; seen {
			continue // first row wins
		}
		units[disp] = Unit{Symbol: disp, Code: code, UnitID: int32(id), DisplayName: disp, Description: desc}
	}
	for _, u := range supplements {
		if _, seen := units[u.Symbol]; !seen {
			units[u.Symbol] = u
		}
	}
}

// LookupUnit returns the table row for a DSL unit symbol.
func LookupUnit(symbol string) (Unit, bool) {
	unitsOnce.Do(loadUnits)
	u, ok := units[symbol]
	return u, ok
}
